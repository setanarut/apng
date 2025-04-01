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
	Images []image.Image // The successive images.

	// The successive delay times, one per frame, in 100ths of a second (centiseconds).
	//
	// Note: For 30 FPS, each frame lasts 1/30 second ≈ 3.33 centiseconds.
	// When using integer delays, you might use 3 centiseconds per frame.
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
		fdat := make([]byte, 4, len(id))
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
}

type pngChunk struct {
	ihdr  []byte
	idats []idat
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
		// todo
	case "tRNS":
		// todo
	case "IDAT":
		c.stage = dsSeenIDAT
		err = c.parseIDAT(length)
	case "IEND":
		c.stage = dsSeenIEND
		err = nil
	}

	c.bb.Next(4) // Get rid of crc(4 bytes).
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

func isSameColorModel(img []image.Image) bool {
	if len(img) == 0 || img[0] == nil {
		return false
	}

	reference := img[0].ColorModel()
	for i := 1; i < len(img); i++ {
		if img[i] == nil || img[i].ColorModel() != reference {
			return false
		}
	}
	return true
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

	if !isSameColorModel(a.Images) {
		return errors.New("apng: must be all the same color model of images")
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

	// Prepare PNG data for all frames in parallel
	type frameData struct {
		index int
		ihdr  []byte
		idats []idat // Changed from [][]byte to []idat
	}

	frameDataChan := make(chan frameData, len(a.Images))
	var wg sync.WaitGroup

	for i, img := range a.Images {
		wg.Add(1)
		go func(index int, image image.Image) {
			defer wg.Done()

			bb := &bytes.Buffer{}
			if err := png.Encode(bb, image); err != nil {
				// Use mutex to handle error state
				mutex.Lock()
				if e.err == nil {
					e.err = errors.New("apng: png encoding error(" + err.Error() + ")")
				}
				mutex.Unlock()
				return
			}

			pc, err := fetchPNGChunk(bb)
			if err != nil {
				mutex.Lock()
				if e.err == nil {
					e.err = err
				}
				mutex.Unlock()
				return
			}

			// fmt.Printf("Frame %d calculated\n", index)

			frameDataChan <- frameData{
				index: index,
				ihdr:  pc.ihdr,
				idats: pc.idats, // The type of idats returned by fetchPNGChunk should be []idat
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

	// Write the first frame and then other frames in correct order
	for i := range a.Images {
		fd, ok := frameDataMap[i]
		if !ok {
			return errors.New("apng: missing frame data for index " + strconv.Itoa(i))
		}

		e.ihdr = fd.ihdr
		e.idats = fd.idats

		// The first image is the default image
		if i == 0 {
			e.writeIHDR()
			e.writeacTL()
			e.writefcTL(i)
			e.writeIDATs()
		} else {
			e.writefcTL(i)
			e.writefdATs()
		}
	}

	e.writeIEND()

	return e.err
}
