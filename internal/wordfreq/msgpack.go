package wordfreq

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

func decodeMsgpack(r io.Reader) (interface{}, error) {
	dec := msgpackDecoder{r: bufio.NewReader(r)}
	return dec.decodeValue()
}

type msgpackDecoder struct {
	r *bufio.Reader
}

func (d *msgpackDecoder) decodeValue() (interface{}, error) {
	b, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch {
	case b <= 0x7f:
		return int64(b), nil
	case b >= 0xe0:
		return int64(int8(b)), nil
	case b >= 0xa0 && b <= 0xbf:
		length := int(b & 0x1f)
		return d.readString(length)
	case b >= 0x90 && b <= 0x9f:
		length := int(b & 0x0f)
		return d.readArray(length)
	case b >= 0x80 && b <= 0x8f:
		length := int(b & 0x0f)
		return d.readMap(length)
	}

	switch b {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xc4:
		length, err := d.readUint8()
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(length))
	case 0xc5:
		length, err := d.readUint16()
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(length))
	case 0xc6:
		length, err := d.readUint32()
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(length))
	case 0xca:
		val, err := d.readUint32()
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(val)), nil
	case 0xcb:
		val, err := d.readUint64()
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(val), nil
	case 0xcc:
		val, err := d.readUint8()
		if err != nil {
			return nil, err
		}
		return int64(val), nil
	case 0xcd:
		val, err := d.readUint16()
		if err != nil {
			return nil, err
		}
		return int64(val), nil
	case 0xce:
		val, err := d.readUint32()
		if err != nil {
			return nil, err
		}
		return int64(val), nil
	case 0xcf:
		val, err := d.readUint64()
		if err != nil {
			return nil, err
		}
		return val, nil
	case 0xd0:
		val, err := d.readInt8()
		if err != nil {
			return nil, err
		}
		return int64(val), nil
	case 0xd1:
		val, err := d.readInt16()
		if err != nil {
			return nil, err
		}
		return int64(val), nil
	case 0xd2:
		val, err := d.readInt32()
		if err != nil {
			return nil, err
		}
		return int64(val), nil
	case 0xd3:
		val, err := d.readInt64()
		if err != nil {
			return nil, err
		}
		return val, nil
	case 0xd9:
		length, err := d.readUint8()
		if err != nil {
			return nil, err
		}
		return d.readString(int(length))
	case 0xda:
		length, err := d.readUint16()
		if err != nil {
			return nil, err
		}
		return d.readString(int(length))
	case 0xdb:
		length, err := d.readUint32()
		if err != nil {
			return nil, err
		}
		return d.readString(int(length))
	case 0xdc:
		length, err := d.readUint16()
		if err != nil {
			return nil, err
		}
		return d.readArray(int(length))
	case 0xdd:
		length, err := d.readUint32()
		if err != nil {
			return nil, err
		}
		return d.readArray(int(length))
	case 0xde:
		length, err := d.readUint16()
		if err != nil {
			return nil, err
		}
		return d.readMap(int(length))
	case 0xdf:
		length, err := d.readUint32()
		if err != nil {
			return nil, err
		}
		return d.readMap(int(length))
	default:
		return nil, fmt.Errorf("unsupported msgpack prefix 0x%x", b)
	}
}

func (d *msgpackDecoder) readArray(length int) ([]interface{}, error) {
	out := make([]interface{}, 0, length)
	for i := 0; i < length; i++ {
		val, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	return out, nil
}

func (d *msgpackDecoder) readMap(length int) (map[interface{}]interface{}, error) {
	out := make(map[interface{}]interface{}, length)
	for i := 0; i < length; i++ {
		key, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		val, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		out[key] = val
	}
	return out, nil
}

func (d *msgpackDecoder) readString(length int) (string, error) {
	data, err := d.readBytes(length)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (d *msgpackDecoder) readBytes(length int) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("invalid length %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (d *msgpackDecoder) readByte() (byte, error) {
	return d.r.ReadByte()
}

func (d *msgpackDecoder) readUint8() (uint8, error) {
	b, err := d.readByte()
	return b, err
}

func (d *msgpackDecoder) readUint16() (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(d.r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

func (d *msgpackDecoder) readUint32() (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(d.r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}

func (d *msgpackDecoder) readUint64() (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(d.r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(buf[:]), nil
}

func (d *msgpackDecoder) readInt8() (int8, error) {
	val, err := d.readUint8()
	return int8(val), err
}

func (d *msgpackDecoder) readInt16() (int16, error) {
	val, err := d.readUint16()
	return int16(val), err
}

func (d *msgpackDecoder) readInt32() (int32, error) {
	val, err := d.readUint32()
	return int32(val), err
}

func (d *msgpackDecoder) readInt64() (int64, error) {
	val, err := d.readUint64()
	return int64(val), err
}
