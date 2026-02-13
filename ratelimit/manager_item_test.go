package ratelimit

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tinylib/msgp/msgp"
)

func TestItem_MessagePackOperations(t *testing.T) {
	t.Parallel()

	t.Run("marshal and unmarshal empty item", func(t *testing.T) {
		original := &item{}

		data, err := original.MarshalMsg(nil)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		unmarshaled := &item{}
		remaining, err := unmarshaled.UnmarshalMsg(data)
		require.NoError(t, err)
		require.Empty(t, remaining)

		require.Equal(t, original.currHits, unmarshaled.currHits)
		require.Equal(t, original.prevHits, unmarshaled.prevHits)
		require.Equal(t, original.exp, unmarshaled.exp)
	})

	t.Run("marshal and unmarshal item with data", func(t *testing.T) {
		original := &item{
			currHits: 5,
			prevHits: 3,
			exp:      1234567890,
		}

		data, err := original.MarshalMsg(nil)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		unmarshaled := &item{}
		remaining, err := unmarshaled.UnmarshalMsg(data)
		require.NoError(t, err)
		require.Empty(t, remaining)

		require.Equal(t, original.currHits, unmarshaled.currHits)
		require.Equal(t, original.prevHits, unmarshaled.prevHits)
		require.Equal(t, original.exp, unmarshaled.exp)
	})

	t.Run("marshal into existing buffer", func(t *testing.T) {
		original := &item{
			currHits: 10,
			prevHits: 7,
			exp:      9876543210,
		}

		existingBuffer := make([]byte, 0, 100)
		data, err := original.MarshalMsg(existingBuffer)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		unmarshaled := &item{}
		remaining, err := unmarshaled.UnmarshalMsg(data)
		require.NoError(t, err)
		require.Empty(t, remaining)

		require.Equal(t, original.currHits, unmarshaled.currHits)
		require.Equal(t, original.prevHits, unmarshaled.prevHits)
		require.Equal(t, original.exp, unmarshaled.exp)
	})

	t.Run("msgsize returns reasonable size", func(t *testing.T) {
		testItem := &item{
			currHits: 100,
			prevHits: 50,
			exp:      1234567890,
		}

		size := testItem.Msgsize()
		require.True(t, size > 0)
		require.True(t, size < 100)
	})
}

func TestItem_DecodeEncodeMsg(t *testing.T) {
	t.Parallel()

	t.Run("encode and decode with msgp.Writer/Reader", func(t *testing.T) {
		original := &item{
			currHits: 15,
			prevHits: 8,
			exp:      4567890123,
		}

		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		err := original.EncodeMsg(writer)
		require.NoError(t, err)

		err = writer.Flush()
		require.NoError(t, err)

		reader := msgp.NewReader(&buf)
		decoded := &item{}

		err = decoded.DecodeMsg(reader)
		require.NoError(t, err)

		require.Equal(t, original.currHits, decoded.currHits)
		require.Equal(t, original.prevHits, decoded.prevHits)
		require.Equal(t, original.exp, decoded.exp)
	})

	t.Run("encode empty item and decode", func(t *testing.T) {
		original := &item{}

		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		err := original.EncodeMsg(writer)
		require.NoError(t, err)

		err = writer.Flush()
		require.NoError(t, err)

		reader := msgp.NewReader(&buf)
		decoded := &item{}

		err = decoded.DecodeMsg(reader)
		require.NoError(t, err)

		require.Equal(t, original.currHits, decoded.currHits)
		require.Equal(t, original.prevHits, decoded.prevHits)
		require.Equal(t, original.exp, decoded.exp)
	})

	t.Run("encode decode roundtrip with max values", func(t *testing.T) {
		original := &item{
			currHits: 999999,
			prevHits: 888888,
			exp:      18446744073709551615,
		}

		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		err := original.EncodeMsg(writer)
		require.NoError(t, err)

		err = writer.Flush()
		require.NoError(t, err)

		reader := msgp.NewReader(&buf)
		decoded := &item{}

		err = decoded.DecodeMsg(reader)
		require.NoError(t, err)

		require.Equal(t, original.currHits, decoded.currHits)
		require.Equal(t, original.prevHits, decoded.prevHits)
		require.Equal(t, original.exp, decoded.exp)
	})

	t.Run("encode decode roundtrip with negative values", func(t *testing.T) {
		original := &item{
			currHits: -100,
			prevHits: -200,
			exp:      1234567890,
		}

		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		err := original.EncodeMsg(writer)
		require.NoError(t, err)

		err = writer.Flush()
		require.NoError(t, err)

		reader := msgp.NewReader(&buf)
		decoded := &item{}

		err = decoded.DecodeMsg(reader)
		require.NoError(t, err)

		require.Equal(t, original.currHits, decoded.currHits)
		require.Equal(t, original.prevHits, decoded.prevHits)
		require.Equal(t, original.exp, decoded.exp)
	})
}

func TestItem_ErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal invalid map header", func(t *testing.T) {
		invalidData := []byte{0xFF, 0xFF}

		item := &item{}
		_, err := item.UnmarshalMsg(invalidData)

		require.Error(t, err)
	})

	t.Run("unmarshal truncated data", func(t *testing.T) {
		original := &item{
			currHits: 5,
			prevHits: 3,
			exp:      1234567890,
		}

		data, err := original.MarshalMsg(nil)
		require.NoError(t, err)

		truncatedData := data[:len(data)/2]

		item := &item{}
		_, err = item.UnmarshalMsg(truncatedData)

		require.Error(t, err)
	})

	t.Run("decode invalid map header from reader", func(t *testing.T) {
		invalidData := []byte{0xFF, 0xFF}
		reader := msgp.NewReader(bytes.NewReader(invalidData))

		item := &item{}
		err := item.DecodeMsg(reader)

		require.Error(t, err)
	})

	t.Run("decode truncated data from reader", func(t *testing.T) {
		original := &item{
			currHits: 5,
			prevHits: 3,
			exp:      1234567890,
		}

		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)
		err := original.EncodeMsg(writer)
		require.NoError(t, err)
		_ = writer.Flush()

		data := buf.Bytes()
		truncatedData := data[:len(data)/2]

		reader := msgp.NewReader(bytes.NewReader(truncatedData))

		item := &item{}
		err = item.DecodeMsg(reader)

		require.Error(t, err)
	})

	t.Run("unmarshal with unknown fields skips them", func(t *testing.T) {
		original := &item{
			currHits: 10,
			prevHits: 5,
			exp:      9876543210,
		}

		data, err := original.MarshalMsg(nil)
		require.NoError(t, err)

		item := &item{}
		remaining, err := item.UnmarshalMsg(data)

		require.NoError(t, err)
		require.Equal(t, 10, item.currHits)
		require.Equal(t, 5, item.prevHits)
		require.Equal(t, uint64(9876543210), item.exp)
		_ = remaining
	})
}

func TestItem_EncodeDecodeErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("decode with empty map", func(t *testing.T) {
		emptyMapData := []byte{0x80}

		reader := msgp.NewReader(bytes.NewReader(emptyMapData))
		testItem := &item{}
		err := testItem.DecodeMsg(reader)

		require.NoError(t, err)
		require.Equal(t, 0, testItem.currHits)
		require.Equal(t, 0, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)
	})

	t.Run("unmarshal with empty map", func(t *testing.T) {
		emptyMapData := []byte{0x80}

		testItem := &item{}
		_, err := testItem.UnmarshalMsg(emptyMapData)

		require.NoError(t, err)
		require.Equal(t, 0, testItem.currHits)
		require.Equal(t, 0, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)
	})

	t.Run("decode with partial fields", func(t *testing.T) {
		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		_ = writer.WriteMapHeader(2)
		_ = writer.WriteString("currHits")
		_ = writer.WriteInt(10)
		_ = writer.WriteString("prevHits")
		_ = writer.WriteInt(5)

		_ = writer.Flush()

		reader := msgp.NewReader(&buf)
		testItem := &item{}
		err := testItem.DecodeMsg(reader)

		require.NoError(t, err)
		require.Equal(t, 10, testItem.currHits)
		require.Equal(t, 5, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)
	})

	t.Run("unmarshal with partial fields", func(t *testing.T) {
		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		_ = writer.WriteMapHeader(2)
		_ = writer.WriteString("currHits")
		_ = writer.WriteInt(10)
		_ = writer.WriteString("prevHits")
		_ = writer.WriteInt(5)

		_ = writer.Flush()

		data := buf.Bytes()
		testItem := &item{}
		_, err := testItem.UnmarshalMsg(data)

		require.NoError(t, err)
		require.Equal(t, 10, testItem.currHits)
		require.Equal(t, 5, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)
	})

	t.Run("decode with only exp field", func(t *testing.T) {
		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		_ = writer.WriteMapHeader(1)
		_ = writer.WriteString("exp")
		_ = writer.WriteUint64(1234567890)

		_ = writer.Flush()

		reader := msgp.NewReader(&buf)
		testItem := &item{}
		err := testItem.DecodeMsg(reader)

		require.NoError(t, err)
		require.Equal(t, 0, testItem.currHits)
		require.Equal(t, 0, testItem.prevHits)
		require.Equal(t, uint64(1234567890), testItem.exp)
	})

	t.Run("encode decode with map containing extra unknown field", func(t *testing.T) {
		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		_ = writer.WriteMapHeader(4)
		_ = writer.WriteString("currHits")
		_ = writer.WriteInt(100)
		_ = writer.WriteString("prevHits")
		_ = writer.WriteInt(50)
		_ = writer.WriteString("exp")
		_ = writer.WriteUint64(9876543210)
		_ = writer.WriteString("unknownField")
		_ = writer.WriteString("someValue")

		_ = writer.Flush()

		data := buf.Bytes()
		testItem := &item{}
		_, err := testItem.UnmarshalMsg(data)

		require.NoError(t, err)
		require.Equal(t, 100, testItem.currHits)
		require.Equal(t, 50, testItem.prevHits)
		require.Equal(t, uint64(9876543210), testItem.exp)
	})

	t.Run("encode decode with map containing only unknown fields", func(t *testing.T) {
		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)

		_ = writer.WriteMapHeader(2)
		_ = writer.WriteString("unknown1")
		_ = writer.WriteString("value1")
		_ = writer.WriteString("unknown2")
		_ = writer.WriteInt(999)

		_ = writer.Flush()

		data := buf.Bytes()
		testItem := &item{}
		_, err := testItem.UnmarshalMsg(data)

		require.NoError(t, err)
		require.Equal(t, 0, testItem.currHits)
		require.Equal(t, 0, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)
	})

	t.Run("encode item with only currHits", func(t *testing.T) {
		testItem := &item{
			currHits: 100,
		}

		var buf bytes.Buffer
		writer := msgp.NewWriter(&buf)
		err := testItem.EncodeMsg(writer)
		require.NoError(t, err)
		_ = writer.Flush()

		data := buf.Bytes()
		require.NotEmpty(t, data)

		decoded := &item{}
		reader := msgp.NewReader(bytes.NewReader(data))
		err = decoded.DecodeMsg(reader)
		require.NoError(t, err)

		require.Equal(t, 100, decoded.currHits)
	})
}
