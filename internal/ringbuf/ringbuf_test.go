package ringbuf

import (
	"fmt"
	"reflect"
	"testing"
)

func assertEqual(found, expected interface{}) {
	if !reflect.DeepEqual(found, expected) {
		panic(fmt.Sprintf("found %#v, expected %#v", found, expected))
	}
}

func testRange(low, hi int) []int {
	result := make([]int, hi-low)
	for i := low; i < hi; i++ {
		result[i-low] = i
	}
	return result
}

func extractContents[T any](buf *Buffer[T]) (result []T) {
	buf.Range(true, func(i *T) bool {
		result = append(result, *i)
		return false
	})
	return result
}

func TestRingbuf(t *testing.T) {
	b := NewExpandableBuffer[int](16, 32)
	numItems := 0
	for i := 0; i < 32; i++ {
		assertEqual(b.Length(), numItems)
		b.Add(i)
		numItems++
	}

	assertEqual(b.Length(), 32)
	assertEqual(extractContents(b), testRange(0, 32))

	for i := 32; i < 40; i++ {
		b.Add(i)
		assertEqual(b.Length(), 32)
	}

	assertEqual(b.Length(), 32)
	assertEqual(extractContents(b), testRange(8, 40))
	assertEqual(b.Length(), 32)

	for i := 39; i >= 8; i-- {
		assertEqual(b.Length(), i-7)
		val, success := b.Pop()
		assertEqual(success, true)
		assertEqual(val, i)
	}

	_, success := b.Pop()
	assertEqual(success, false)
}

func TestClear(t *testing.T) {
	b := NewExpandableBuffer[int](8, 8)
	for i := 1; i <= 4; i++ {
		b.Add(i)
	}
	assertEqual(extractContents(b), testRange(1, 5))
	b.Clear()
	assertEqual(b.Length(), 0)
	assertEqual(len(extractContents(b)), 0)
	// verify that the internal storage was cleared (important for GC)
	for i := 0; i < len(b.buffer); i++ {
		assertEqual(b.buffer[i], 0)
	}
}
