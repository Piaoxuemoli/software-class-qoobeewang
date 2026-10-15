package huffman

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"testing"
)

// roundtrip 压缩 data 再解压，返回解压结果。
func roundtrip(t *testing.T, data []byte) ([]byte, []byte) {
	t.Helper()
	var comp bytes.Buffer
	w := NewWriter(&comp)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := io.ReadAll(NewReader(bytes.NewReader(comp.Bytes())))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("往返不一致: got %d bytes, want %d", len(got), len(data))
	}
	return comp.Bytes(), got
}

func TestRoundTrip(t *testing.T) {
	t.Run("TC-HUFF-001 空输入", func(t *testing.T) {
		roundtrip(t, nil)
	})

	t.Run("TC-HUFF-002 单字节", func(t *testing.T) {
		roundtrip(t, []byte{0xAB})
	})

	t.Run("TC-HUFF-003 全同字节（单符号码）", func(t *testing.T) {
		roundtrip(t, bytes.Repeat([]byte{0x00}, 100_000))
	})

	t.Run("TC-HUFF-004 随机数据跨块往返", func(t *testing.T) {
		rng := rand.New(rand.NewSource(42))
		data := make([]byte, 2*BlockSize+12345)
		rng.Read(data)
		roundtrip(t, data)
	})

	t.Run("TC-HUFF-005 多次小块写入", func(t *testing.T) {
		var comp bytes.Buffer
		w := NewWriter(&comp)
		rng := rand.New(rand.NewSource(7))
		want := make([]byte, 0, 100_000)
		for range 100 {
			chunk := make([]byte, 1000)
			rng.Read(chunk)
			want = append(want, chunk...)
			if _, err := w.Write(chunk); err != nil {
				t.Fatalf("Write: %v", err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		got, err := io.ReadAll(NewReader(&comp))
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("往返不一致")
		}
	})

	t.Run("TC-HUFF-006 可压缩数据变小", func(t *testing.T) {
		data := bytes.Repeat([]byte("hello world "), 10_000)
		comp, _ := roundtrip(t, data)
		if len(comp) >= len(data) {
			t.Errorf("压缩后 %d 字节，未小于原始 %d 字节", len(comp), len(data))
		}
	})

	t.Run("TC-HUFF-007 恰好一块边界", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))
		data := make([]byte, BlockSize)
		rng.Read(data)
		roundtrip(t, data)
	})
}

// crafted 构造一个手工数据块流。
func crafted(origLen, compLen uint32, lengths [256]uint8, comp []byte) []byte {
	buf := make([]byte, 0, headerLen+len(comp)+4)
	buf = binary.LittleEndian.AppendUint32(buf, origLen)
	buf = binary.LittleEndian.AppendUint32(buf, compLen)
	buf = append(buf, lengths[:]...)
	buf = append(buf, comp...)
	return buf
}

func TestCorrupt(t *testing.T) {
	var lens [256]uint8
	lens['a'], lens['b'] = 1, 1

	cases := []struct {
		name  string
		input []byte
	}{
		{"TC-HUFF-008 空流", nil},
		{"TC-HUFF-009 块头截断", []byte{1, 0}},
		{"TC-HUFF-010 origLen 超过上限", crafted(BlockSize+1, 1, lens, []byte{0})},
		{"TC-HUFF-011 compLen 为 0", crafted(10, 0, lens, nil)},
		{"TC-HUFF-012 compLen 超上限", crafted(10, 81, lens, nil)},
		{"TC-HUFF-013 compLen 小于位流最小值", crafted(9, 1, lens, []byte{0})},
		{"TC-HUFF-014 码表为空", crafted(1, 1, [256]uint8{}, []byte{0})},
		{"TC-HUFF-015 码表过订阅", func() []byte {
			var l [256]uint8
			l['a'], l['b'], l['c'] = 1, 1, 1
			return crafted(1, 1, l, []byte{0})
		}()},
		{"TC-HUFF-016 码长超上限", func() []byte {
			var l [256]uint8
			l['a'] = 65
			return crafted(1, 1, l, []byte{0})
		}()},
		{"TC-HUFF-017 位流中途耗尽", crafted(5, 1, func() [256]uint8 {
			var l [256]uint8
			l['a'], l['b'] = 2, 2
			return l
		}(), []byte{0x00})},
		{"TC-HUFF-018 压缩数据截断", func() []byte {
			b := crafted(100, 13, func() [256]uint8 {
				var l [256]uint8
				l['a'] = 1
				return l
			}(), make([]byte, 13))
			return b[:len(b)-3]
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := io.ReadAll(NewReader(bytes.NewReader(tc.input)))
			if !errors.Is(err, ErrCorrupt) {
				t.Errorf("err = %v, want ErrCorrupt", err)
			}
		})
	}
}

func TestTruncatedValidStream(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	data := make([]byte, 5000)
	rng.Read(data)
	var comp bytes.Buffer
	w := NewWriter(&comp)
	w.Write(data)
	w.Close()

	t.Run("TC-HUFF-019 无结束标记截断", func(t *testing.T) {
		truncated := comp.Bytes()[:len(comp.Bytes())-4] // 去掉结束块
		_, err := io.ReadAll(NewReader(bytes.NewReader(truncated)))
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("err = %v, want ErrCorrupt", err)
		}
	})

	t.Run("TC-HUFF-020 块中途截断", func(t *testing.T) {
		truncated := comp.Bytes()[:len(comp.Bytes())/2]
		_, err := io.ReadAll(NewReader(bytes.NewReader(truncated)))
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("err = %v, want ErrCorrupt", err)
		}
	})
}

func FuzzDecode(f *testing.F) {
	var comp bytes.Buffer
	w := NewWriter(&comp)
	rng := rand.New(rand.NewSource(9))
	seed := make([]byte, 4096)
	rng.Read(seed)
	w.Write(seed)
	w.Close()
	f.Add(comp.Bytes())
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		// 任意输入不得 panic、不得死循环；错误必须是 ErrCorrupt 或 io.EOF。
		_, err := io.ReadAll(NewReader(bytes.NewReader(data)))
		if err != nil && !errors.Is(err, ErrCorrupt) {
			t.Fatalf("err = %v, want ErrCorrupt or nil", err)
		}
	})
}

func benchmarkData(n int) []byte {
	// 混合可压缩数据：半结构化重复 + 噪声。
	rng := rand.New(rand.NewSource(5))
	data := make([]byte, n)
	words := [][]byte{[]byte("alpha"), []byte("beta"), []byte("gamma"), []byte("delta")}
	for i := 0; i < len(data); {
		w := words[rng.Intn(len(words))]
		i += copy(data[i:], w)
	}
	return data
}

func BenchmarkEncode(b *testing.B) {
	data := benchmarkData(BlockSize)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		w := NewWriter(io.Discard)
		w.Write(data)
		w.Close()
	}
}

func BenchmarkDecode(b *testing.B) {
	data := benchmarkData(BlockSize)
	var comp bytes.Buffer
	w := NewWriter(&comp)
	w.Write(data)
	w.Close()
	enc := comp.Bytes()
	b.ReportMetric(float64(len(enc))/float64(len(data)), "ratio")
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		if _, err := io.Copy(io.Discard, NewReader(bytes.NewReader(enc))); err != nil {
			b.Fatal(err)
		}
	}
}
