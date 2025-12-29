package util_test

import (
	"slices"
	"testing"

	"github.com/anton15x/dropbox_ignore_service/src/util"
	"github.com/stretchr/testify/require"
)

func TestStringSet(t *testing.T) {
	sort := func(arr []string) []string {
		slices.Sort(arr)
		return arr
	}

	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{
			name: "add_different_values",
			f: func(t *testing.T) {
				set := util.NewStringSet()
				require.Equal(t, true, set.Add("B"))
				require.Equal(t, []string{"B"}, sort(set.Values()))
				require.Equal(t, true, set.Add("A"))
				require.Equal(t, []string{"A", "B"}, sort(set.Values()))
				require.Equal(t, true, set.Add("C"))
				require.Equal(t, []string{"A", "B", "C"}, sort(set.Values()))
			},
		},
		{
			name: "add_existing_values",
			f: func(t *testing.T) {
				set := util.NewStringSet()
				require.Equal(t, true, set.Add("A"))
				require.Equal(t, false, set.Add("A"))
				require.Equal(t, true, set.Add("B"))
				require.Equal(t, false, set.Add("B"))
				require.Equal(t, []string{"A", "B"}, sort(set.Values()))
				require.Equal(t, false, set.Add("A"))
				require.Equal(t, []string{"A", "B"}, sort(set.Values()))
			},
		},
		{
			name: "remove_values",
			f: func(t *testing.T) {
				set := util.NewStringSet()
				require.Equal(t, true, set.Add("A"))
				require.Equal(t, true, set.Add("B"))
				require.Equal(t, true, set.Add("C"))
				require.Equal(t, true, set.Add("D"))
				require.Equal(t, true, set.Add("E"))
				require.Equal(t, []string{"A", "B", "C", "D", "E"}, sort(set.Values()))
				require.Equal(t, true, set.Remove("B"))
				require.Equal(t, []string{"A", "C", "D", "E"}, sort(set.Values()))
				require.Equal(t, true, set.Remove("A"))
				require.Equal(t, []string{"C", "D", "E"}, sort(set.Values()))
				require.Equal(t, true, set.Remove("E"))
				require.Equal(t, []string{"C", "D"}, sort(set.Values()))
				require.Equal(t, false, set.Remove("E"))
				require.Equal(t, false, set.Remove("E"))
				require.Equal(t, false, set.Remove("E"))
			},
		},
		{
			name: "listener",
			f: func(t *testing.T) {
				set := util.NewStringSet()

				expectedAdded := []string{}
				expectedRemove := []string{}
				changeCalls := 0

				// add change listener first
				// the change listener so gets called first
				// than the add or remove lister get called, that check if the change calls was made
				set.AddChangeEventListener(func() {
					require.Zero(t, changeCalls)
					changeCalls++
				})
				set.AddAddEventListener(func(s string) {
					require.True(t, len(expectedAdded) > 0)
					first := expectedAdded[0]
					expectedAdded = expectedAdded[1:]
					require.Equal(t, first, s)

					changeCalls--
					require.Zero(t, changeCalls)
				})
				set.AddRemoveEventListener(func(s string) {
					require.True(t, len(expectedRemove) > 0)
					first := expectedRemove[0]
					expectedRemove = expectedRemove[1:]
					require.Equal(t, first, s)

					changeCalls--
					require.Zero(t, changeCalls)
				})

				expectedAdded = append(expectedAdded, "A")
				require.Equal(t, true, set.Add("A"))
				expectedAdded = append(expectedAdded, "B")
				require.Equal(t, true, set.Add("B"))
				expectedAdded = append(expectedAdded, "C")
				require.Equal(t, true, set.Add("C"))
				expectedAdded = append(expectedAdded, "D")
				require.Equal(t, true, set.Add("D"))
				expectedAdded = append(expectedAdded, "E")
				require.Equal(t, true, set.Add("E"))
				require.Equal(t, []string{"A", "B", "C", "D", "E"}, sort(set.Values()))
				require.Equal(t, false, set.Add("E"))
				require.Equal(t, false, set.Add("E"))
				require.Equal(t, false, set.Add("E"))
				expectedRemove = append(expectedRemove, "B")
				require.Equal(t, true, set.Remove("B"))
				require.Equal(t, []string{"A", "C", "D", "E"}, sort(set.Values()))
				expectedRemove = append(expectedRemove, "A")
				require.Equal(t, true, set.Remove("A"))
				require.Equal(t, []string{"C", "D", "E"}, sort(set.Values()))
				expectedRemove = append(expectedRemove, "E")
				require.Equal(t, true, set.Remove("E"))
				require.Equal(t, []string{"C", "D"}, sort(set.Values()))
				require.Equal(t, false, set.Remove("E"))
				require.Equal(t, false, set.Remove("E"))
				require.Equal(t, false, set.Remove("E"))

				require.Len(t, expectedAdded, 0)
				require.Len(t, expectedRemove, 0)
				require.Zero(t, changeCalls)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f(t)
		})
	}
}
