package wordfreq

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractWordlistOrderAndFilter(t *testing.T) {
	data := encodeTestMsgpack([]interface{}{
		[]interface{}{5.0, []interface{}{"hello", "a", "go-1"}},
		[]interface{}{4.0, []interface{}{"world", "go"}},
	})

	wheelPath := writeTestWheel(t, map[string][]byte{
		"wordfreq/data/large_en.msgpack": data,
	})

	words, err := ExtractWordlist(wheelPath, "en", "large", 3)
	if err != nil {
		t.Fatalf("ExtractWordlist failed: %v", err)
	}

	expected := []string{"hello", "world", "go"}
	if len(words) != len(expected) {
		t.Fatalf("expected %d words, got %d", len(expected), len(words))
	}
	for i, word := range expected {
		if words[i] != word {
			t.Fatalf("expected %q at index %d, got %q", word, i, words[i])
		}
	}
}

func TestExtractWordlistLimit(t *testing.T) {
	data := encodeTestMsgpack([]interface{}{
		[]interface{}{5.0, []interface{}{"hello", "world", "again"}},
		[]interface{}{4.0, []interface{}{"more", "words"}},
	})
	wheelPath := writeTestWheel(t, map[string][]byte{
		"wordfreq/data/large_en.msgpack": data,
	})

	words, err := ExtractWordlist(wheelPath, "en", "large", 2)
	if err != nil {
		t.Fatalf("ExtractWordlist failed: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
}

func encodeTestMsgpack(value interface{}) []byte {
	var buf bytes.Buffer
	writeMsgpack(&buf, value)
	return buf.Bytes()
}

func writeMsgpack(buf *bytes.Buffer, value interface{}) {
	switch v := value.(type) {
	case nil:
		buf.WriteByte(0xc0)
	case bool:
		if v {
			buf.WriteByte(0xc3)
		} else {
			buf.WriteByte(0xc2)
		}
	case int:
		writeMsgpack(buf, int64(v))
	case int64:
		if v >= 0 && v <= 0x7f {
			buf.WriteByte(byte(v))
			return
		}
		buf.WriteByte(0xd3)
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], uint64(v))
		buf.Write(tmp[:])
	case float64:
		buf.WriteByte(0xcb)
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], math.Float64bits(v))
		buf.Write(tmp[:])
	case string:
		writeMsgpackString(buf, v)
	case []interface{}:
		writeMsgpackArray(buf, v)
	case []string:
		items := make([]interface{}, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
		writeMsgpackArray(buf, items)
	default:
		panic("unsupported type in test msgpack encoder")
	}
}

func writeMsgpackArray(buf *bytes.Buffer, values []interface{}) {
	length := len(values)
	if length <= 15 {
		buf.WriteByte(0x90 | byte(length))
	} else {
		buf.WriteByte(0xdc)
		var tmp [2]byte
		binary.BigEndian.PutUint16(tmp[:], uint16(length))
		buf.Write(tmp[:])
	}
	for _, value := range values {
		writeMsgpack(buf, value)
	}
}

func writeMsgpackString(buf *bytes.Buffer, value string) {
	length := len(value)
	if length <= 31 {
		buf.WriteByte(0xa0 | byte(length))
	} else {
		buf.WriteByte(0xd9)
		buf.WriteByte(byte(length))
	}
	buf.WriteString(value)
}

func TestWriteAttribution(t *testing.T) {
	wheelPath := writeTestWheel(t, map[string][]byte{
		"wordfreq-1.0.0.dist-info/LICENSE": []byte("Apache License"),
	})

	outDir := t.TempDir()
	if err := WriteAttribution(wheelPath, outDir); err != nil {
		t.Fatalf("WriteAttribution failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "ATTRIBUTION.txt")); err != nil {
		t.Fatalf("expected ATTRIBUTION.txt: %v", err)
	}
	license, err := os.ReadFile(filepath.Join(outDir, "LICENSE.txt"))
	if err != nil {
		t.Fatalf("expected LICENSE.txt: %v", err)
	}
	if string(license) != "Apache License" {
		t.Fatalf("unexpected license contents: %s", string(license))
	}
}

func TestListLanguages(t *testing.T) {
	files := map[string][]byte{
		"wordfreq/data/large_en.msgpack.gz":         []byte("x"),
		"wordfreq/data/large_pt-br.msgpack.gz":      []byte("x"),
		"wordfreq/data/small_zh-cn.msgpack.gz":      []byte("x"),
		"wordfreq/data/_chinese_mapping.msgpack.gz": []byte("x"),
		"wordfreq/data/jieba_zh.txt":                []byte("x"),
	}
	wheelPath := writeTestWheel(t, files)

	types, err := ListLanguageTypes(wheelPath)
	if err != nil {
		t.Fatalf("ListLanguageTypes failed: %v", err)
	}
	langs := LanguagesFromTypes(types)
	expected := []string{"en", "pt-br", "zh-cn"}
	if len(langs) != len(expected) {
		t.Fatalf("expected %d langs, got %d", len(expected), len(langs))
	}
	for i, lang := range expected {
		if langs[i] != lang {
			t.Fatalf("expected %q at index %d, got %q", lang, i, langs[i])
		}
	}
}

func writeTestWheel(t *testing.T, files map[string][]byte) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "wordfreq-*.whl")
	if err != nil {
		t.Fatalf("failed to create temp wheel: %v", err)
	}
	defer func() {
		_ = tmpFile.Close()
	}()

	zw := zip.NewWriter(tmpFile)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("failed to write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return tmpFile.Name()
}
