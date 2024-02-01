package main_test

import (
	"testing"

	main "github.com/anton15x/dropbox_ignore_service"
	"github.com/stretchr/testify/require"
)

func TestSortedStringSet(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{
			name: "add_different_values",
			f: func(t *testing.T) {
				set := main.NewSortedStringSet()
				require.Equal(t, true, set.Add("B"))
				require.Equal(t, []string{"B"}, set.Values)
				require.Equal(t, true, set.Add("A"))
				require.Equal(t, []string{"A", "B"}, set.Values)
				require.Equal(t, true, set.Add("C"))
				require.Equal(t, []string{"A", "B", "C"}, set.Values)
			},
		},
		{
			name: "add_existing_values",
			f: func(t *testing.T) {
				set := main.NewSortedStringSet()
				require.Equal(t, true, set.Add("A"))
				require.Equal(t, false, set.Add("A"))
				require.Equal(t, true, set.Add("B"))
				require.Equal(t, false, set.Add("B"))
				require.Equal(t, []string{"A", "B"}, set.Values)
				require.Equal(t, false, set.Add("A"))
				require.Equal(t, []string{"A", "B"}, set.Values)
			},
		},
		{
			name: "remove_values",
			f: func(t *testing.T) {
				set := main.NewSortedStringSet()
				require.Equal(t, true, set.Add("A"))
				require.Equal(t, true, set.Add("B"))
				require.Equal(t, true, set.Add("C"))
				require.Equal(t, true, set.Add("D"))
				require.Equal(t, true, set.Add("E"))
				require.Equal(t, []string{"A", "B", "C", "D", "E"}, set.Values)
				require.Equal(t, true, set.Remove("B"))
				require.Equal(t, []string{"A", "C", "D", "E"}, set.Values)
				require.Equal(t, true, set.Remove("A"))
				require.Equal(t, []string{"C", "D", "E"}, set.Values)
				require.Equal(t, true, set.Remove("E"))
				require.Equal(t, []string{"C", "D"}, set.Values)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f(t)
		})
	}
}
