package mp4meta

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	mp4lib "github.com/abema/go-mp4"
	"github.com/aler9/writerseeker"
	"github.com/sunfish-shogi/bufseekio"
)

type mp4Writer interface {
	Write(p []byte) (int, error)
	Seek(offset int64, whence int) (int64, error)
	StartBox(bi *mp4lib.BoxInfo) (*mp4lib.BoxInfo, error)
	EndBox() (*mp4lib.BoxInfo, error)
	CopyBox(r io.ReadSeeker, bi *mp4lib.BoxInfo) error
}

type mp4WriteSeeker interface {
	Write(p []byte) (int, error)
	Seek(offset int64, whence int) (int64, error)
	Bytes() []byte
}

// Make new atoms and write to.
func createAndWrite(w mp4Writer, ctx mp4lib.Context, _tags *MP4Tag) error {
	var boxData *mp4lib.Data
	dataCtx := ctx
	dataCtx.UnderIlstMeta = true
	for boxType, tagName := range atomsMap {
		switch tagName {
		case "BPM":
			if _tags.BPM == 0 {
				continue
			}
			buf := make([]byte, 2)
			temp := make([]byte, 2)
			binary.BigEndian.PutUint16(temp, uint16(_tags.BPM))
			buf = append(temp, buf...)
			boxData = &mp4lib.Data{
				DataType: mp4lib.DataTypeSignedIntBigEndian,
				Data:     buf,
			}

		case "TrackNumber", "DiscNumber":
			total := strings.ReplaceAll(tagName, "Number", "Total")
			numVal := reflect.ValueOf(*_tags).FieldByName(tagName).Int()
			totalVal := reflect.ValueOf(*_tags).FieldByName(total).Int()
			if numVal == 0 && totalVal == 0 {
				continue
			}
			buf := make([]byte, 2)
			temp := make([]byte, 2)
			binary.BigEndian.PutUint16(temp, uint16(numVal))
			buf = append(buf, temp...)
			binary.BigEndian.PutUint16(temp, uint16(totalVal))
			buf = append(buf, temp...)
			buf = append(buf, make([]byte, 2)...)
			boxData = &mp4lib.Data{
				DataType: mp4lib.DataTypeBinary,
				Data:     buf,
			}

		case "CoverArt":
			if _tags.CoverArt == nil {
				continue
			}
			buf := new(bytes.Buffer)
			if err := png.Encode(buf, *_tags.CoverArt); err != nil {
				return err
			}
			boxData = &mp4lib.Data{
				DataType: 13,
				Data:     buf.Bytes(),
			}

		default:
			if reflect.ValueOf(*_tags).FieldByName(tagName).IsZero() {
				continue
			}
			val := reflect.ValueOf(*_tags).FieldByName(tagName).String()
			boxData = &mp4lib.Data{
				DataType: mp4lib.DataTypeStringUTF8,
				Data:     []byte(val),
			}
		}

		if _, err := w.StartBox(&mp4lib.BoxInfo{Type: boxType}); err != nil {
			return err
		}
		if _, err := w.StartBox(&mp4lib.BoxInfo{Type: mp4lib.BoxTypeData()}); err != nil {
			return err
		}

		if _, err := mp4lib.Marshal(w, boxData, dataCtx); err != nil {
			return err
		}

		if _, err := w.EndBox(); err != nil {
			return err
		}
		if _, err := w.EndBox(); err != nil {
			return err
		}
	}
	return nil

}

func containsAtom(boxType mp4lib.BoxType) mp4lib.BoxType {
	if _, ok := atomsMap[boxType]; ok {
		return boxType
	}
	return mp4lib.BoxType{}
}

func saveMP4(r io.ReadSeeker, wo io.Writer, w mp4Writer, ws mp4WriteSeeker, _tags *MP4Tag) error {
	var mdatOffsetDiff int64
	var stcoOffsets []int64
	var ilstExists bool
	rs := bufseekio.NewReadSeeker(r, 1024*1024, 4)

	_, err := mp4lib.ReadBoxStructure(rs, func(h *mp4lib.ReadHandle) (interface{}, error) {
		switch h.BoxInfo.Type {
		// 1. moov, trak, mdia, minf, stbl, udta
		case mp4lib.BoxTypeMoov(),
			mp4lib.BoxTypeTrak(),
			mp4lib.BoxTypeMdia(),
			mp4lib.BoxTypeMinf(),
			mp4lib.BoxTypeStbl(),
			mp4lib.BoxTypeUdta(),
			mp4lib.BoxTypeMeta(),
			mp4lib.BoxTypeIlst():
			_, err := w.StartBox(&h.BoxInfo)
			if err != nil {
				return nil, err
			}
			if h.BoxInfo.Type != mp4lib.BoxTypeIlst() {
				if _, err := h.Expand(); err != nil {
					return nil, err
				}
			}
			// 1-a. [only moov box] add udta box if not exists
			if h.BoxInfo.Type == mp4lib.BoxTypeMoov() && !ilstExists {
				path := []mp4lib.BoxType{mp4lib.BoxTypeUdta(), mp4lib.BoxTypeMeta()}
				for _, boxType := range path {
					if _, err := w.StartBox(&mp4lib.BoxInfo{Type: boxType}); err != nil {
						return nil, err
					}
				}
				ctx := h.BoxInfo.Context
				ctx.UnderUdta = true
				if _, err := w.StartBox(&mp4lib.BoxInfo{Type: mp4lib.BoxTypeHdlr()}); err != nil {
					return nil, err
				}
				hdlr := &mp4lib.Hdlr{
					HandlerType: [4]byte{'m', 'd', 'i', 'r'},
				}
				if _, err := mp4lib.Marshal(w, hdlr, ctx); err != nil {
					return nil, err
				}
				if _, err := w.EndBox(); err != nil {
					return nil, err
				}
				if _, err := w.StartBox(&mp4lib.BoxInfo{Type: mp4lib.BoxTypeIlst()}); err != nil {
					return nil, err
				}
				ctx.UnderIlst = true
				if err := createAndWrite(w, ctx, _tags); err != nil {
					return nil, err
				}
				if _, err := w.EndBox(); err != nil {
					return nil, err
				}
				for range path {
					if _, err := w.EndBox(); err != nil {
						return nil, err
					}
				}
			}
			// 1-b. [only ilst box] add metadatas
			if h.BoxInfo.Type == mp4lib.BoxTypeIlst() {
				ctx := h.BoxInfo.Context
				ctx.UnderIlst = true
				if err := createAndWrite(w, ctx, _tags); err != nil {
					return nil, err
				}
				ilstExists = true
			}
			if _, err = w.EndBox(); err != nil {
				return nil, err
			}
		// 2. otherwise
		default:
			// 2-a. [only stco box] keep offset
			if h.BoxInfo.Type == mp4lib.BoxTypeStco() {
				offset, _ := w.Seek(0, io.SeekCurrent)
				stcoOffsets = append(stcoOffsets, offset)
			}
			// 2-b. [only mdat box] keep difference of offsets
			if h.BoxInfo.Type == mp4lib.BoxTypeMdat() {
				iOffset := int64(h.BoxInfo.Offset)
				oOffset, _ := w.Seek(0, io.SeekCurrent)
				mdatOffsetDiff = oOffset - iOffset
			}
			// copy box without modification
			if err := w.CopyBox(r, &h.BoxInfo); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		return err
	}

	ts := bufseekio.NewReadSeeker(bytes.NewReader(ws.Bytes()), 1024*1024, 3)

	if _, err = ws.Seek(0, io.SeekStart); err != nil {
		return err
	}
	// if mdat box is moved, update stco box
	if mdatOffsetDiff != 0 {
		for _, stcoOffset := range stcoOffsets {
			// seek to stco box header
			if _, err := ts.Seek(stcoOffset, io.SeekStart); err != nil {
				return err
			}
			// read box header
			bi, err := mp4lib.ReadBoxInfo(ts)
			if err != nil {
				return err
			}
			// read stco box payload
			var stco mp4lib.Stco
			_, err = mp4lib.Unmarshal(ts, bi.Size-bi.HeaderSize, &stco, bi.Context)
			if err != nil {
				return err
			}
			// update chunk offsets
			for i := range stco.ChunkOffset {
				stco.ChunkOffset[i] += uint32(mdatOffsetDiff)
			}
			// seek to stco box payload
			_, err = bi.SeekToPayload(ws)
			if err != nil {
				return err
			}
			// write stco box payload
			if _, err := mp4lib.Marshal(ws, &stco, bi.Context); err != nil {
				return err
			}
		}
	}
	if _, err = ws.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if reflect.TypeOf(wo) == reflect.TypeOf(new(os.File)) {
		f := wo.(*os.File)
		path, err := filepath.Abs(f.Name())
		if err != nil {
			return err
		}
		w2, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer w2.Close()
		if _, err = w2.Write(ws.Bytes()); err != nil {
			return err
		}
		if _, err = f.Seek(0, io.SeekEnd); err != nil {
			return err
		}
		return nil
	}
	if _, err := wo.Write(ws.Bytes()); err != nil {
		return err
	}
	return nil

}

func SaveMP4(r io.ReadSeeker, wo io.Writer, _tags *MP4Tag) error {
	ws := &writerseeker.WriterSeeker{}
	defer ws.Close()
	w := mp4lib.NewWriter(ws)
	return saveMP4(r, wo, w, ws, _tags)
}
