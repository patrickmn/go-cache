package cache

import "testing"

func TestIncrementWithInt(t *testing.T) {
	tc := NewOrderedCache[string, int](DefaultExpiration, 0)
	tc.Set("tint", 1, DefaultExpiration)
	n, err := tc.Increment("tint", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if *n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tint")
	if !found {
		t.Error("tint was not found")
	}
	if *x != 3 {
		t.Error("tint is not 3:", x)
	}
}

func TestIncrementInt8(t *testing.T) {
	tc := NewOrderedCache[string, int8](DefaultExpiration, 0)
	tc.Set("int8", 1, DefaultExpiration)
	n, err := tc.Increment("int8", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if *n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("int8")
	if !found {
		t.Error("int8 was not found")
	}
	if *x != 3 {
		t.Error("int8 is not 3:", x)
	}
}

func TestIncrementOverflowInt(t *testing.T) {
	tc := NewOrderedCache[string, int8](DefaultExpiration, 0)
	tc.Set("int8", 127, DefaultExpiration)
	n, err := tc.Increment("int8", 1)
	if err != nil {
		t.Error("Error incrementing int8:", err)
	}
	if *n != -128 {
		t.Error("Returned number is not -128:", n)
	}
	x, _ := tc.Get("int8")
	int8 := *x
	if int8 != -128 {
		t.Error("int8 did not overflow as expected; value:", int8)
	}

}

func TestIncrementOverflowUint(t *testing.T) {
	tc := NewOrderedCache[string, uint8](DefaultExpiration, 0)
	tc.Set("uint8", 255, DefaultExpiration)
	n, err := tc.Increment("uint8", 1)
	if err != nil {
		t.Error("Error incrementing int8:", err)
	}
	if *n != 0 {
		t.Error("Returned number is not 0:", n)
	}
	x, _ := tc.Get("uint8")
	uint8 := *x
	if uint8 != 0 {
		t.Error("uint8 did not overflow as expected; value:", uint8)
	}
}

func BenchmarkIncrement(b *testing.B) {
	b.StopTimer()
	tc := NewOrderedCache[string, int](DefaultExpiration, 0)
	tc.Set("foo", 0, DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Increment("foo", 1)
	}
}
