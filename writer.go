// apng is Animated PNG (APNG) encoder
package apng

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"strconv"
	"sync"
)

type idat []byte

const pngHeader string = "\x89PNG\r\n\x1a\n"

const (
	dsStart = iota
	dsSeenIHDR
	dsSeenPLTE
	dsSeentRNS
	dsSeenIDAT
	dsSeenIEND
)

func writeUint16(b []uint8, u uint16) {
	b[0] = uint8(u >> 8)
	b[1] = uint8(u)
}

func writeUint32(b []uint8, u uint32) {
	b[0] = uint8(u >> 24)
	b[1] = uint8(u >> 16)
	b[2] = uint8(u >> 8)
	b[3] = uint8(u)
}

// APNG encapsulates animated PNG frames, their delays, disposal methods, loop count, and global configuration.
type APNG struct {
	// The successive images.
	//
	// Images obtained via SubImage() are not supported, If an image is a sub-image, copy it into a new image before encoding.
	Images []image.Image

	// The successive delay times, one per frame, in 100ths of a second (centiseconds).
	Delays    []uint16
	Disposals []byte // The successive disposal methods, one per frame.
	LoopCount uint32 // The loop count. 0 indicates infinite looping.
	Config    image.Config
}
type encoder struct {
	aPNG   *APNG
	writer io.Writer
	seqNum uint32 // Sequence number of the animation chunk.

	tmpHeader [8]byte
	tmp       [4 * 256]byte
	tmpFooter [4]byte

	ihdr  []byte
	idats []idat

	err error
}

func (e *encoder) writeChunk(b []byte, name string) {
	if e.err != nil {
		return
	}

	// Write header (length, type).
	n := uint32(len(b))
	if int(n) != len(b) {
		e.err = errors.New("apng: chunk is too large")
		return
	}
	writeUint32(e.tmpHeader[:4], n)
	e.tmpHeader[4] = name[0]
	e.tmpHeader[5] = name[1]
	e.tmpHeader[6] = name[2]
	e.tmpHeader[7] = name[3]
	_, e.err = e.writer.Write(e.tmpHeader[:8])
	if e.err != nil {
		return
	}

	// Write data.
	_, e.err = e.writer.Write(b)
	if e.err != nil {
		return
	}

	// Write footer (crc).
	crc := crc32.NewIEEE()
	crc.Write(e.tmpHeader[4:8])
	crc.Write(b)
	writeUint32(e.tmpFooter[:4], crc.Sum32())
	_, e.err = e.writer.Write(e.tmpFooter[:4])
}

func (e *encoder) writeIHDR() {
	e.writeChunk(e.ihdr, "IHDR")
}

func (e *encoder) writeacTL() {
	writeUint32(e.tmp[0:4], uint32(len(e.aPNG.Images)))
	writeUint32(e.tmp[4:8], e.aPNG.LoopCount)
	e.writeChunk(e.tmp[:8], "acTL")
}

func (e *encoder) writefcTL(frameIndex int) {
	writeUint32(e.tmp[0:4], e.seqNum) // Write sequence_number.
	bounds := (e.aPNG.Images[frameIndex]).Bounds()
	writeUint32(e.tmp[4:8], uint32(bounds.Max.X-bounds.Min.X))  // Write width.
	writeUint32(e.tmp[8:12], uint32(bounds.Max.Y-bounds.Min.Y)) // Write height.
	writeUint32(e.tmp[12:16], uint32(bounds.Min.X))             // Write x_offset.
	writeUint32(e.tmp[16:20], uint32(bounds.Min.Y))             // Write y_offset.
	writeUint16(e.tmp[20:22], e.aPNG.Delays[frameIndex])        // Write delay_num(numerator).
	writeUint16(e.tmp[22:24], uint16(100))                      // Write delay_den(denominator).
	e.tmp[24] = 0
	if e.aPNG.Disposals != nil {
		e.tmp[24] = e.aPNG.Disposals[frameIndex] // Write dispose_op
	}
	e.tmp[25] = 0 // Write blend_op.

	e.writeChunk(e.tmp[:26], "fcTL")
	e.seqNum++
}

func (e *encoder) writeIDATs() {
	for _, id := range e.idats {
		e.writeChunk(id, "IDAT")
	}
}

func (e *encoder) writefdATs() {
	for _, id := range e.idats {
		writeUint32(e.tmp[0:4], e.seqNum)
		fdat := make([]byte, 4, len(id)+4)
		copy(fdat, e.tmp[0:4])
		fdat = append(fdat, id...)
		e.writeChunk(fdat, "fdAT")
		e.seqNum++
	}
}

func (e *encoder) writeIEND() {
	e.writeChunk(nil, "IEND")
}

type chunkFetcher struct {
	bb    *bytes.Buffer
	tmp   [3 * 256]byte
	stage int
	pc    *pngChunk
	plte  []byte // PLTE chunk data
	trns  []byte // tRNS chunk data
}

type pngChunk struct {
	ihdr  []byte
	idats []idat
	plte  []byte
	trns  []byte
}

func (c *chunkFetcher) parseIHDR(length uint32) error {
	_, err := io.ReadFull(c.bb, c.tmp[:length])
	if err != nil {
		return err
	}
	c.pc.ihdr = make([]byte, length)
	copy(c.pc.ihdr, c.tmp[:length])
	return nil
}

func (c *chunkFetcher) parseIDAT(length uint32) error {
	id := c.bb.Next(int(length))
	if len(id) < int(length) {
		return io.EOF
	}
	c.pc.idats = append(c.pc.idats, id)
	return nil
}

func (c *chunkFetcher) parsePLTE(length uint32) error {
	data := make([]byte, length)
	_, err := io.ReadFull(c.bb, data)
	if err != nil {
		return err
	}

	c.plte = make([]byte, length+8) // 8 bytes: length (4) + chunk type (4)
	writeUint32(c.plte[:4], length)
	copy(c.plte[4:8], []byte("PLTE"))
	copy(c.plte[8:], data)

	// Store PLTE in the pngChunk if available
	if c.pc != nil {
		c.pc.plte = make([]byte, length)
		copy(c.pc.plte, data)
	}
	return nil
}

func (c *chunkFetcher) parsetRNS(length uint32) error {
	data := make([]byte, length)
	_, err := io.ReadFull(c.bb, data)
	if err != nil {
		return err
	}

	c.trns = make([]byte, length+8) // 8 bytes: length (4) + chunk type (4)
	writeUint32(c.trns[:4], length)
	copy(c.trns[4:8], []byte("tRNS"))
	copy(c.trns[8:], data)

	// Store tRNS in the pngChunk if available
	if c.pc != nil {
		c.pc.trns = make([]byte, length)
		copy(c.pc.trns, data)
	}
	return nil
}

func (c *chunkFetcher) parsePNGChunk() error {
	_, err := io.ReadFull(c.bb, c.tmp[:8])
	if err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(c.tmp[:4])

	switch string(c.tmp[4:8]) {
	case "IHDR":
		c.stage = dsSeenIHDR
		err = c.parseIHDR(length)
	case "PLTE":
		c.stage = dsSeenPLTE
		err = c.parsePLTE(length)
	case "tRNS":
		c.stage = dsSeentRNS
		err = c.parsetRNS(length)
	case "IDAT":
		c.stage = dsSeenIDAT
		err = c.parseIDAT(length)
	case "IEND":
		c.stage = dsSeenIEND
		err = nil
	default:
		// Skip other chunks
		c.bb.Next(int(length))
	}

	c.bb.Next(4) // Get rid of crc(4 bytes).
	return err
}

func (c *chunkFetcher) parsePNGChunkWithPalette() error {
	_, err := io.ReadFull(c.bb, c.tmp[:8])
	if err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(c.tmp[:4])

	switch string(c.tmp[4:8]) {
	case "IHDR":
		c.stage = dsSeenIHDR
		_, err = io.ReadFull(c.bb, c.tmp[:length]) // Just read but don't store
	case "PLTE":
		c.stage = dsSeenPLTE
		err = c.parsePLTE(length)
	case "tRNS":
		c.stage = dsSeentRNS
		err = c.parsetRNS(length)
	case "IDAT":
		c.stage = dsSeenIDAT
		c.bb.Next(int(length)) // Skip IDAT content
	case "IEND":
		c.stage = dsSeenIEND
	default:
		c.bb.Next(int(length)) // Skip other chunks
	}

	c.bb.Next(4) // Skip CRC
	return err
}

func fetchPNGChunk(bb *bytes.Buffer) (*pngChunk, error) {
	bb.Next(len(pngHeader))
	c := &chunkFetcher{
		bb:    bb,
		stage: dsStart,
		pc:    new(pngChunk),
	}

	for c.stage != dsSeenIEND {
		if err := c.parsePNGChunk(); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
	}
	return c.pc, nil
}

// fetchPaletteChunk extracts PLTE and tRNS chunks from paletted images
func fetchPaletteChunk(bb *bytes.Buffer) (plte []byte, trns []byte, err error) {
	bb.Next(len(pngHeader))
	c := &chunkFetcher{
		bb:    bb,
		stage: dsStart,
	}

	for c.stage != dsSeenIEND {
		if err := c.parsePNGChunkWithPalette(); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, nil, err
		}
	}
	return c.plte, c.trns, nil
}

// encodePalettedImage encodes a paletted image for APNG format
func encodePalettedImage(img *image.Paletted) (*pngChunk, error) {
	bb := &bytes.Buffer{}
	if err := png.Encode(bb, img); err != nil {
		return nil, errors.New("apng: palette encoding error: " + err.Error())
	}

	return fetchPNGChunk(bb)
}

func fullfillFrameRegionConstraints(img []image.Image) bool {
	if len(img) == 0 || img[0] == nil {
		return false
	}
	reference := img[0].Bounds()
	// constraints:
	if !(reference.Min.X >= 0 && reference.Min.Y >= 0) {
		return false
	}
	for i := 1; i < len(img); i++ {
		if img[i] == nil {
			return false
		}
		bounds := img[i].Bounds()
		if !(bounds.Min.X >= 0 && bounds.Min.Y >= 0 && bounds.Max.X <= reference.Max.X && bounds.Max.Y <= reference.Max.Y) {
			return false
		}
	}
	return true
}

// EncodeAll encodes the entire APNG struct to the io.Writer, validating input constraints.
func EncodeAll(w io.Writer, a *APNG) error {
	if len(a.Images) == 0 {
		return errors.New("apng: need at least one image")
	}
	if len(a.Images) != len(a.Delays) {
		return errors.New("apng: mismatched image and delay lengths")
	}
	if a.Disposals != nil && len(a.Images) != len(a.Disposals) {
		return errors.New("apng: mismatch image and disposal lengths")
	}
	if !fullfillFrameRegionConstraints(a.Images) {
		return errors.New("apng: must fullfill frame region constraints")
	}

	e := encoder{
		aPNG:   a,
		writer: w,
	}

	_, e.err = io.WriteString(w, pngHeader)

	// Data to be used while processing the first image
	var mutex sync.Mutex
	var hasFirstPaletted bool
	var globalPLTE, globalTRNS []byte

	// Check if the first image is paletted
	if firstImg, ok := a.Images[0].(*image.Paletted); ok {
		hasFirstPaletted = true

		// Extract PLTE and tRNS chunks from the first paletted image
		bb := &bytes.Buffer{}
		if err := png.Encode(bb, firstImg); err != nil {
			return errors.New("apng: png encoding error(" + err.Error() + ")")
		}

		var err error
		globalPLTE, globalTRNS, err = fetchPaletteChunk(bb)
		if err != nil {
			return err
		}
	}

	// Prepare PNG data for all frames in parallel
	type frameData struct {
		index      int
		ihdr       []byte
		idats      []idat
		isPaletted bool
	}

	frameDataChan := make(chan frameData, len(a.Images))
	var wg sync.WaitGroup

	for i, img := range a.Images {
		wg.Add(1)
		go func(index int, img image.Image) {
			defer wg.Done()

			// Check for paletted image
			paletted, isPaletted := img.(*image.Paletted)

			var pc *pngChunk
			var err error

			if isPaletted {
				pc, err = encodePalettedImage(paletted)
			} else {
				bb := &bytes.Buffer{}
				if err := png.Encode(bb, img); err != nil {
					mutex.Lock()
					if e.err == nil {
						e.err = errors.New("apng: png encoding error(" + err.Error() + ")")
					}
					mutex.Unlock()
					return
				}
				pc, err = fetchPNGChunk(bb)
			}

			if err != nil {
				mutex.Lock()
				if e.err == nil {
					e.err = err
				}
				mutex.Unlock()
				return
			}

			frameDataChan <- frameData{
				index:      index,
				ihdr:       pc.ihdr,
				idats:      pc.idats,
				isPaletted: isPaletted,
			}
		}(i, img)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(frameDataChan)
	}()

	// Write output in correct order
	frameDataMap := make(map[int]frameData)
	for fd := range frameDataChan {
		frameDataMap[fd.index] = fd
	}

	// Error check
	if e.err != nil {
		return e.err
	}

	// Setup for the first frame
	fd, ok := frameDataMap[0]
	if !ok {
		return errors.New("apng: missing frame data for index 0")
	}

	e.ihdr = fd.ihdr
	e.idats = fd.idats

	e.writeIHDR()

	// Write PLTE and tRNS chunks for paletted images
	if hasFirstPaletted && globalPLTE != nil {
		e.writeChunk(globalPLTE[8:8+binary.BigEndian.Uint32(globalPLTE[:4])], "PLTE")

		// Write tRNS only if it exists
		if globalTRNS != nil {
			e.writeChunk(globalTRNS[8:8+binary.BigEndian.Uint32(globalTRNS[:4])], "tRNS")
		}
	}

	e.writeacTL()
	e.writefcTL(0)
	e.writeIDATs()

	// Process other frames
	for i := 1; i < len(a.Images); i++ {
		fd, ok := frameDataMap[i]
		if !ok {
			return errors.New("apng: missing frame data for index " + strconv.Itoa(i))
		}

		e.ihdr = fd.ihdr
		e.idats = fd.idats

		e.writefcTL(i)
		e.writefdATs()
	}

	e.writeIEND()

	return e.err
}
