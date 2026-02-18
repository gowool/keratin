package internal

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterceptors_Apply(t *testing.T) {
	tests := []struct {
		name                string
		interceptors        Interceptors[int]
		input               int
		want                int
		wantCancelCallCount int
	}{
		{
			name:                "empty interceptors returns input unchanged",
			interceptors:        Interceptors[int]{},
			input:               42,
			want:                42,
			wantCancelCallCount: 0,
		},
		{
			name:                "nil interceptors returns input unchanged",
			interceptors:        nil,
			input:               42,
			want:                42,
			wantCancelCallCount: 0,
		},
		{
			name: "single interceptor that modifies value",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) {
					return n * 2, nil
				},
			},
			input:               5,
			want:                10,
			wantCancelCallCount: 0,
		},
		{
			name: "single interceptor with cancel function",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) {
					callCount := 0
					return n + 10, func() {
						callCount++
					}
				},
			},
			input:               5,
			want:                15,
			wantCancelCallCount: 1,
		},
		{
			name: "multiple interceptors chain modifications",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) { return n + 1, nil },
				func(n int) (int, func()) { return n * 2, nil },
				func(n int) (int, func()) { return n - 5, nil },
			},
			input:               10,
			want:                17,
			wantCancelCallCount: 0,
		},
		{
			name: "multiple interceptors with multiple cancels",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) {
					return n + 1, func() {}
				},
				func(n int) (int, func()) {
					return n * 2, nil
				},
				func(n int) (int, func()) {
					return n - 5, func() {}
				},
			},
			input:               10,
			want:                17,
			wantCancelCallCount: 2,
		},
		{
			name: "all interceptors have cancel functions",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) { return n + 1, func() {} },
				func(n int) (int, func()) { return n * 2, func() {} },
				func(n int) (int, func()) { return n - 5, func() {} },
			},
			input:               10,
			want:                17,
			wantCancelCallCount: 3,
		},
		{
			name: "interceptors with zero input",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) { return n + 100, nil },
			},
			input:               0,
			want:                100,
			wantCancelCallCount: 0,
		},
		{
			name: "interceptors with negative numbers",
			interceptors: Interceptors[int]{
				func(n int) (int, func()) { return -n, nil },
			},
			input:               -42,
			want:                42,
			wantCancelCallCount: 0,
		},
		{
			name: "large number of interceptors",
			interceptors: func() Interceptors[int] {
				interceptors := make(Interceptors[int], 100)
				for i := range 100 {
					interceptors[i] = func(n int) (int, func()) {
						return n + 1, func() {}
					}
				}
				return interceptors
			}(),
			input:               0,
			want:                100,
			wantCancelCallCount: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cancel := tt.interceptors.Apply(tt.input)
			require.Equal(t, tt.want, got)

			if tt.wantCancelCallCount > 0 {
				require.NotNil(t, cancel)
				require.NotPanics(t, func() {
					cancel()
				})
			} else {
				if cancel != nil {
					require.NotPanics(t, func() {
						cancel()
					})
				}
			}
		})
	}
}

func TestInterceptors_Apply_CancelOrder(t *testing.T) {
	t.Run("cancel functions called in reverse order", func(t *testing.T) {
		var callOrder []int

		interceptors := Interceptors[string]{
			func(s string) (string, func()) {
				return s, func() {
					callOrder = append(callOrder, 1)
				}
			},
			func(s string) (string, func()) {
				return s, func() {
					callOrder = append(callOrder, 2)
				}
			},
			func(s string) (string, func()) {
				return s, func() {
					callOrder = append(callOrder, 3)
				}
			},
		}

		_, cancel := interceptors.Apply("test")
		cancel()

		require.Equal(t, []int{3, 2, 1}, callOrder)
	})

	t.Run("cancel called multiple times", func(t *testing.T) {
		var callCount int32

		interceptors := Interceptors[int]{
			func(n int) (int, func()) {
				return n, func() {
					atomic.AddInt32(&callCount, 1)
				}
			},
		}

		_, cancel := interceptors.Apply(1)
		cancel()
		cancel()
		cancel()

		require.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	})
}

func TestInterceptors_Apply_WithStructType(t *testing.T) {
	type TestStruct struct {
		Value int
		Name  string
	}

	tests := []struct {
		name         string
		interceptors Interceptors[TestStruct]
		input        TestStruct
		want         TestStruct
	}{
		{
			name: "modify struct fields",
			interceptors: Interceptors[TestStruct]{
				func(s TestStruct) (TestStruct, func()) {
					s.Value += 10
					return s, nil
				},
				func(s TestStruct) (TestStruct, func()) {
					s.Name = "modified"
					return s, nil
				},
			},
			input: TestStruct{Value: 5, Name: "original"},
			want:  TestStruct{Value: 15, Name: "modified"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.interceptors.Apply(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestInterceptors_Apply_WithPointerType(t *testing.T) {
	type TestStruct struct {
		Value int
		Name  string
	}

	tests := []struct {
		name         string
		interceptors Interceptors[*TestStruct]
		input        *TestStruct
		want         *TestStruct
	}{
		{
			name: "modify struct through pointer",
			interceptors: Interceptors[*TestStruct]{
				func(s *TestStruct) (*TestStruct, func()) {
					s.Value *= 2
					return s, nil
				},
				func(s *TestStruct) (*TestStruct, func()) {
					s.Name += " updated"
					return s, nil
				},
			},
			input: &TestStruct{Value: 10, Name: "test"},
			want:  &TestStruct{Value: 20, Name: "test updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.interceptors.Apply(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestInterceptors_Apply_WithStringType(t *testing.T) {
	tests := []struct {
		name         string
		interceptors Interceptors[string]
		input        string
		want         string
	}{
		{
			name: "string concatenation",
			interceptors: Interceptors[string]{
				func(s string) (string, func()) {
					return s + " world", nil
				},
			},
			input: "hello",
			want:  "hello world",
		},
		{
			name: "string transformations",
			interceptors: Interceptors[string]{
				func(s string) (string, func()) {
					return s + "-suffix", nil
				},
				func(s string) (string, func()) {
					return "prefix-" + s, nil
				},
			},
			input: "middle",
			want:  "prefix-middle-suffix",
		},
		{
			name: "empty string",
			interceptors: Interceptors[string]{
				func(s string) (string, func()) {
					return "not empty", nil
				},
			},
			input: "",
			want:  "not empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.interceptors.Apply(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestInterceptors_Apply_CancelBehavior(t *testing.T) {
	t.Run("nil cancel functions are not stored", func(t *testing.T) {
		interceptors := Interceptors[int]{
			func(n int) (int, func()) { return n + 1, nil },
			func(n int) (int, func()) { return n + 2, nil },
			func(n int) (int, func()) { return n + 3, func() {} },
		}

		_, cancel := interceptors.Apply(0)
		cancel()

		require.NotPanics(t, func() {
			cancel()
		})
	})

	t.Run("cancel panics gracefully on nil", func(t *testing.T) {
		interceptors := Interceptors[int]{}
		_, cancel := interceptors.Apply(0)

		require.NotPanics(t, func() {
			if cancel != nil {
				cancel()
			}
		})
	})

	t.Run("multiple calls to cancel execute all cancels each time", func(t *testing.T) {
		var totalCalls int32

		interceptors := Interceptors[int]{
			func(n int) (int, func()) {
				return n, func() {
					atomic.AddInt32(&totalCalls, 1)
				}
			},
			func(n int) (int, func()) {
				return n, func() {
					atomic.AddInt32(&totalCalls, 1)
				}
			},
		}

		_, cancel := interceptors.Apply(0)
		cancel()
		require.Equal(t, int32(2), atomic.LoadInt32(&totalCalls))

		cancel()
		require.Equal(t, int32(4), atomic.LoadInt32(&totalCalls))
	})
}

func TestInterceptors_Apply_InterfaceType(t *testing.T) {
	t.Run("interface type with concrete implementations", func(t *testing.T) {
		interceptors := Interceptors[any]{
			func(v any) (any, func()) {
				if s, ok := v.(string); ok {
					return s + " processed", nil
				}
				return v, nil
			},
			func(v any) (any, func()) {
				if s, ok := v.(string); ok {
					return s + " twice", nil
				}
				return v, nil
			},
		}

		got, _ := interceptors.Apply("test")
		require.Equal(t, "test processed twice", got)
	})
}
