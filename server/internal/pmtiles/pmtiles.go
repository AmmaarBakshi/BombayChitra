// Package pmtiles reads tiles from a local PMTiles v3 archive (https://github.com/protomaps/PMTiles).
//
// The archive is a single file: a fixed header, a root directory, optional leaf directories and tile data. Lookups
// are a binary search in an in-memory directory followed by one positioned read, so serving a tile costs a single
// pread(2) with no tile server or database involved.
package pmtiles

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

// Compression codes from the PMTiles spec.
const (
	CompressionUnknown = 0
	CompressionNone    = 1
	CompressionGzip    = 2
	CompressionBrotli  = 3
	CompressionZstd    = 4
)

// Tile type codes from the PMTiles spec.
const (
	TileTypeUnknown = 0
	TileTypeMVT     = 1
	TileTypePNG     = 2
	TileTypeJPEG    = 3
	TileTypeWebP    = 4
	TileTypeAVIF    = 5
)

const headerLen = 127

// Header is the decoded 127-byte PMTiles v3 header.
type Header struct {
	RootOffset, RootLength         uint64
	MetadataOffset, MetadataLength uint64
	LeafOffset, LeafLength         uint64
	DataOffset, DataLength         uint64
	AddressedTiles, TileEntries    uint64
	TileContents                   uint64
	Clustered                      bool
	InternalCompression            uint8
	TileCompression                uint8
	TileType                       uint8
	MinZoom, MaxZoom               uint8
	MinLon, MinLat, MaxLon, MaxLat float64
	CenterZoom                     uint8
	CenterLon, CenterLat           float64
}

type entry struct {
	TileID    uint64
	Offset    uint64
	Length    uint32
	RunLength uint32
}

// Reader serves tiles from one archive. It is safe for concurrent use.
type Reader struct {
	f        *os.File
	size     int64
	header   Header
	root     []entry
	metadata []byte
	leaves   sync.Map // offset → []entry
}

// Open reads the header, root directory and metadata of the archive at path.
func Open(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	r := &Reader{f: f, size: st.Size()}
	if err := r.init(); err != nil {
		f.Close()
		return nil, fmt.Errorf("pmtiles %s: %w", path, err)
	}
	return r, nil
}

func (r *Reader) init() error {
	buf := make([]byte, headerLen)
	if _, err := r.f.ReadAt(buf, 0); err != nil {
		return err
	}
	h, err := decodeHeader(buf)
	if err != nil {
		return err
	}
	r.header = h
	if r.root, err = r.readDirectory(h.RootOffset, h.RootLength); err != nil {
		return fmt.Errorf("root directory: %w", err)
	}
	raw := make([]byte, h.MetadataLength)
	if _, err := r.f.ReadAt(raw, int64(h.MetadataOffset)); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	if r.metadata, err = decompress(raw, h.InternalCompression); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	return nil
}

// Close releases the file handle.
func (r *Reader) Close() error { return r.f.Close() }

// Header returns the archive header.
func (r *Reader) Header() Header { return r.header }

// Metadata returns the decompressed JSON metadata blob.
func (r *Reader) Metadata() []byte { return r.metadata }

// Size returns the archive size in bytes.
func (r *Reader) Size() int64 { return r.size }

// File exposes the underlying file for range-request downloads.
func (r *Reader) File() io.ReaderAt { return r.f }

// Tile returns the tile bytes exactly as stored (compressed with Header().TileCompression). ok is false when the tile
// is not in the archive.
func (r *Reader) Tile(z uint8, x, y uint32) (data []byte, ok bool, err error) {
	if z < r.header.MinZoom || z > r.header.MaxZoom {
		return nil, false, nil
	}
	id, err := ZXYToID(z, x, y)
	if err != nil {
		return nil, false, err
	}
	dir := r.root
	for depth := 0; depth < 4; depth++ {
		e, found := findEntry(dir, id)
		if !found {
			return nil, false, nil
		}
		if e.RunLength > 0 {
			buf := make([]byte, e.Length)
			if _, err := r.f.ReadAt(buf, int64(r.header.DataOffset+e.Offset)); err != nil {
				return nil, false, err
			}
			return buf, true, nil
		}
		dir, err = r.leaf(r.header.LeafOffset+e.Offset, uint64(e.Length))
		if err != nil {
			return nil, false, err
		}
	}
	return nil, false, errors.New("pmtiles: directory too deep")
}

func (r *Reader) leaf(offset, length uint64) ([]entry, error) {
	if v, ok := r.leaves.Load(offset); ok {
		return v.([]entry), nil
	}
	dir, err := r.readDirectory(offset, length)
	if err != nil {
		return nil, err
	}
	r.leaves.Store(offset, dir)
	return dir, nil
}

func (r *Reader) readDirectory(offset, length uint64) ([]entry, error) {
	raw := make([]byte, length)
	if _, err := r.f.ReadAt(raw, int64(offset)); err != nil {
		return nil, err
	}
	data, err := decompress(raw, r.header.InternalCompression)
	if err != nil {
		return nil, err
	}
	return decodeDirectory(data)
}

func findEntry(entries []entry, id uint64) (entry, bool) {
	// Largest entry with TileID <= id.
	i := sort.Search(len(entries), func(i int) bool { return entries[i].TileID > id }) - 1
	if i < 0 {
		return entry{}, false
	}
	e := entries[i]
	if e.RunLength == 0 { // leaf directory pointer
		return e, true
	}
	if id-e.TileID < uint64(e.RunLength) {
		return e, true
	}
	return entry{}, false
}

func decodeHeader(b []byte) (Header, error) {
	if len(b) < headerLen || string(b[0:7]) != "PMTiles" {
		return Header{}, errors.New("not a PMTiles archive")
	}
	if b[7] != 3 {
		return Header{}, fmt.Errorf("unsupported PMTiles version %d", b[7])
	}
	u64 := func(o int) uint64 { return binary.LittleEndian.Uint64(b[o:]) }
	e7 := func(o int) float64 { return float64(int32(binary.LittleEndian.Uint32(b[o:]))) / 1e7 }
	return Header{
		RootOffset: u64(8), RootLength: u64(16),
		MetadataOffset: u64(24), MetadataLength: u64(32),
		LeafOffset: u64(40), LeafLength: u64(48),
		DataOffset: u64(56), DataLength: u64(64),
		AddressedTiles: u64(72), TileEntries: u64(80), TileContents: u64(88),
		Clustered:           b[96] == 1,
		InternalCompression: b[97], TileCompression: b[98], TileType: b[99],
		MinZoom: b[100], MaxZoom: b[101],
		MinLon: e7(102), MinLat: e7(106), MaxLon: e7(110), MaxLat: e7(114),
		CenterZoom: b[118], CenterLon: e7(119), CenterLat: e7(123),
	}, nil
}

func decodeDirectory(data []byte) ([]entry, error) {
	rd := bytes.NewReader(data)
	n, err := binary.ReadUvarint(rd)
	if err != nil {
		return nil, err
	}
	if n > uint64(len(data)) {
		return nil, errors.New("corrupt directory")
	}
	entries := make([]entry, n)
	var last uint64
	for i := range entries {
		v, err := binary.ReadUvarint(rd)
		if err != nil {
			return nil, err
		}
		last += v
		entries[i].TileID = last
	}
	for i := range entries {
		v, err := binary.ReadUvarint(rd)
		if err != nil {
			return nil, err
		}
		entries[i].RunLength = uint32(v)
	}
	for i := range entries {
		v, err := binary.ReadUvarint(rd)
		if err != nil {
			return nil, err
		}
		entries[i].Length = uint32(v)
	}
	for i := range entries {
		v, err := binary.ReadUvarint(rd)
		if err != nil {
			return nil, err
		}
		if v == 0 && i > 0 {
			entries[i].Offset = entries[i-1].Offset + uint64(entries[i-1].Length)
		} else {
			entries[i].Offset = v - 1
		}
	}
	return entries, nil
}

func decompress(b []byte, compression uint8) ([]byte, error) {
	switch compression {
	case CompressionNone, CompressionUnknown:
		return b, nil
	case CompressionGzip:
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return io.ReadAll(zr)
	}
	return nil, fmt.Errorf("unsupported internal compression %d", compression)
}

// Gunzip decompresses a gzip-compressed tile for clients that do not accept gzip.
func Gunzip(b []byte) ([]byte, error) { return decompress(b, CompressionGzip) }

// ZXYToID converts tile coordinates to a PMTiles Hilbert tile id.
func ZXYToID(z uint8, x, y uint32) (uint64, error) {
	if z > 31 {
		return 0, errors.New("zoom too large")
	}
	n := uint64(1) << z
	if uint64(x) >= n || uint64(y) >= n {
		return 0, errors.New("tile out of range")
	}
	var acc uint64
	for i := uint8(0); i < z; i++ {
		acc += uint64(1) << (2 * i)
	}
	tx, ty := uint64(x), uint64(y)
	var d uint64
	for s := n / 2; s > 0; s /= 2 {
		var rx, ry uint64
		if tx&s > 0 {
			rx = 1
		}
		if ty&s > 0 {
			ry = 1
		}
		d += s * s * ((3 * rx) ^ ry)
		if ry == 0 {
			if rx == 1 {
				tx = s - 1 - tx
				ty = s - 1 - ty
			}
			tx, ty = ty, tx
		}
	}
	return acc + d, nil
}
