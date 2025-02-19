package query

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/segmentio/parquet-go"
	"github.com/stretchr/testify/require"

	"github.com/grafana/pyroscope/pkg/iter"
)

type RepeatedTestRow struct {
	List []int64
}

type testRowGetter struct {
	RowNum int64
}

func (t testRowGetter) RowNumber() int64 {
	return t.RowNum
}

func Test_RepeatedIterator(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rows     []testRowGetter
		rgs      [][]RepeatedTestRow
		expected []RepeatedRow[testRowGetter]
		readSize int
	}{
		{
			name: "single row group no repeated and repeated",
			rows: []testRowGetter{
				{0},
				{1},
				{2},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 1, 1, 1}},
					{[]int64{2}},
					{[]int64{3, 4}},
				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1), parquet.ValueOf(1), parquet.ValueOf(1), parquet.ValueOf(1)}},
				{testRowGetter{1}, []parquet.Value{parquet.ValueOf(2)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(3), parquet.ValueOf(4)}},
			},
		},
		{
			name: "multiple row group no repeated skip group and page",
			rows: []testRowGetter{
				{0},
				{2},
				{7},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1}},
					{[]int64{2}},
					{[]int64{3}},
				},
				{
					{[]int64{4}},
					{[]int64{5}},
					{[]int64{6}},
				},
				{
					{[]int64{7}},
					{[]int64{8}},
					{[]int64{9}},
				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(3)}},
				{testRowGetter{7}, []parquet.Value{parquet.ValueOf(8)}},
			},
		},
		{
			name: "single row group",
			rows: []testRowGetter{
				{0},
				{1},
				{2},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 2, 3}},
					{[]int64{4, 5, 6}},
					{[]int64{7, 8, 9}},
				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1), parquet.ValueOf(2), parquet.ValueOf(3)}},
				{testRowGetter{1}, []parquet.Value{parquet.ValueOf(4), parquet.ValueOf(5), parquet.ValueOf(6)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(7), parquet.ValueOf(8), parquet.ValueOf(9)}},
			},
		},
		{
			name: "skip row group",
			rows: []testRowGetter{
				{0}, {1}, {2}, {6}, {7}, {8},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 2, 3}},
					{[]int64{4, 5, 6}},
					{[]int64{7, 8, 9}},
				},
				{
					{[]int64{10, 11, 12}},
					{[]int64{13, 14, 15}},
					{[]int64{16, 17, 18}},
				},
				{
					{[]int64{19, 20, 21}},
					{[]int64{22, 23, 24}},
					{[]int64{25, 26, 27}},
				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1), parquet.ValueOf(2), parquet.ValueOf(3)}},
				{testRowGetter{1}, []parquet.Value{parquet.ValueOf(4), parquet.ValueOf(5), parquet.ValueOf(6)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(7), parquet.ValueOf(8), parquet.ValueOf(9)}},
				{testRowGetter{6}, []parquet.Value{parquet.ValueOf(19), parquet.ValueOf(20), parquet.ValueOf(21)}},
				{testRowGetter{7}, []parquet.Value{parquet.ValueOf(22), parquet.ValueOf(23), parquet.ValueOf(24)}},
				{testRowGetter{8}, []parquet.Value{parquet.ValueOf(25), parquet.ValueOf(26), parquet.ValueOf(27)}},
			},
		},
		{
			name: "single row group skip through page",
			rows: []testRowGetter{
				{1},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 2, 3}},
					{[]int64{4, 5, 6}},
					{[]int64{7, 8, 9}},
				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{1}, []parquet.Value{parquet.ValueOf(4), parquet.ValueOf(5), parquet.ValueOf(6)}},
			},
		},
		{
			name: "multiple row group skip within page",
			rows: []testRowGetter{
				{0},
				{2},
				{5},
				{7},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 2, 3}}, // 0
					{[]int64{4, 5, 6}},
					{[]int64{7, 8, 9}}, // 2
					{[]int64{0, 0, 0}},
					{[]int64{0, 0, 0}},
				},
				{
					{[]int64{10, 11, 12}}, // 5
					{[]int64{0, 0, 0}},
					{[]int64{13, 14, 15}}, // 7

				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1), parquet.ValueOf(2), parquet.ValueOf(3)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(7), parquet.ValueOf(8), parquet.ValueOf(9)}},
				{testRowGetter{5}, []parquet.Value{parquet.ValueOf(10), parquet.ValueOf(11), parquet.ValueOf(12)}},
				{testRowGetter{7}, []parquet.Value{parquet.ValueOf(13), parquet.ValueOf(14), parquet.ValueOf(15)}},
			},
		},
		{
			name: "multiple row group skip within and through pages and row group",
			rows: []testRowGetter{
				{0},
				{2},
				{8},
				{10},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 2, 3}}, // 0
					{[]int64{4, 5, 6}},
					{[]int64{7, 8, 9}}, // 2
					{[]int64{0, 0, 0}},
					{[]int64{0, 0, 0}},
				},
				{
					{[]int64{0, 0, 0}},
					{[]int64{0, 0, 0}},
					{[]int64{0, 0, 0}},
				},
				{
					{[]int64{10, 11, 12}}, // 8
					{[]int64{0, 0, 0}},
					{[]int64{13, 14, 15}}, // 10

				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1), parquet.ValueOf(2), parquet.ValueOf(3)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(7), parquet.ValueOf(8), parquet.ValueOf(9)}},
				{testRowGetter{8}, []parquet.Value{parquet.ValueOf(10), parquet.ValueOf(11), parquet.ValueOf(12)}},
				{testRowGetter{10}, []parquet.Value{parquet.ValueOf(13), parquet.ValueOf(14), parquet.ValueOf(15)}},
			},
		},
		{
			name: "multiple row group skip within and through pages and row group mix repeated",
			rows: []testRowGetter{
				{0},
				{2},
				{8},
				{10},
			},
			rgs: [][]RepeatedTestRow{
				{
					{[]int64{1, 2, 3}}, // 0
					{[]int64{4, 5}},
					{[]int64{7}}, // 2
					{[]int64{0}},
					{[]int64{0, 0, 0}},
				},
				{
					{[]int64{0, 0, 0}},
					{[]int64{0, 0, 0}},
					{[]int64{0, 0, 0}},
				},
				{
					{[]int64{10, 11, 12}}, // 8
					{[]int64{0, 0, 0}},
					{[]int64{13, 14}}, // 10

				},
			},
			expected: []RepeatedRow[testRowGetter]{
				{testRowGetter{0}, []parquet.Value{parquet.ValueOf(1), parquet.ValueOf(2), parquet.ValueOf(3)}},
				{testRowGetter{2}, []parquet.Value{parquet.ValueOf(7)}},
				{testRowGetter{8}, []parquet.Value{parquet.ValueOf(10), parquet.ValueOf(11), parquet.ValueOf(12)}},
				{testRowGetter{10}, []parquet.Value{parquet.ValueOf(13), parquet.ValueOf(14)}},
			},
		},
	} {
		tc := tc
		for _, readSize := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 10000} {
			tc.readSize = readSize
			t.Run(tc.name+fmt.Sprintf("_rs_%d", readSize), func(t *testing.T) {
				var groups []parquet.RowGroup
				for _, rg := range tc.rgs {
					buffer := parquet.NewBuffer()
					for _, row := range rg {
						require.NoError(t, buffer.Write(row))
					}
					groups = append(groups, buffer)
				}
				actual := readPageIterator(t,
					NewRepeatedPageIterator(
						context.Background(), iter.NewSliceIterator(tc.rows), groups, 0, tc.readSize))
				if diff := cmp.Diff(tc.expected, actual, int64ParquetComparer()); diff != "" {
					t.Errorf("result mismatch (-want +got):\n%s", diff)
				}
			})
		}

	}
}

type MultiRepeatedItem struct {
	X int64
	Y int64
}

type MultiRepeatedTestRow struct {
	List []MultiRepeatedItem
}

func Test_MultiRepeatedPageIterator(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rows     []testRowGetter
		rgs      [][]MultiRepeatedTestRow
		expected []MultiRepeatedRow[testRowGetter]
	}{
		{
			name: "single row group",
			rows: []testRowGetter{
				{0},
			},
			rgs: [][]MultiRepeatedTestRow{
				{
					{
						List: []MultiRepeatedItem{
							{1, 2},
							{3, 4},
							{5, 6},
						},
					},
				},
			},
			expected: []MultiRepeatedRow[testRowGetter]{
				{
					testRowGetter{0},
					[][]parquet.Value{
						{parquet.ValueOf(1), parquet.ValueOf(3), parquet.ValueOf(5)},
						{parquet.ValueOf(2), parquet.ValueOf(4), parquet.ValueOf(6)},
					},
				},
			},
		},
		{
			name: "row group and page seek",
			rows: []testRowGetter{
				{1},
				{4},
				{7},
			},
			rgs: [][]MultiRepeatedTestRow{
				{
					{List: []MultiRepeatedItem{{0, 0}, {0, 0}}},
					{List: []MultiRepeatedItem{{1, 2}, {3, 4}}}, // 1
					{List: []MultiRepeatedItem{{0, 0}, {0, 0}}},
				},
				{
					{List: []MultiRepeatedItem{{0, 0}, {0, 0}}},
					{List: []MultiRepeatedItem{{5, 6}, {7, 8}}}, // 4
					{List: []MultiRepeatedItem{{0, 0}, {0, 0}}},
					{List: []MultiRepeatedItem{{0, 0}, {0, 0}}},
					{List: []MultiRepeatedItem{{9, 10}}}, // 7
				},
			},
			expected: []MultiRepeatedRow[testRowGetter]{
				{
					testRowGetter{1},
					[][]parquet.Value{
						{parquet.ValueOf(1), parquet.ValueOf(3)},
						{parquet.ValueOf(2), parquet.ValueOf(4)},
					},
				},
				{
					testRowGetter{4},
					[][]parquet.Value{
						{parquet.ValueOf(5), parquet.ValueOf(7)},
						{parquet.ValueOf(6), parquet.ValueOf(8)},
					},
				},
				{
					testRowGetter{7},
					[][]parquet.Value{
						{parquet.ValueOf(9)},
						{parquet.ValueOf(10)},
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var groups []parquet.RowGroup
			for _, rg := range tc.rgs {
				buffer := parquet.NewBuffer()
				for _, row := range rg {
					require.NoError(t, buffer.Write(row))
				}
				groups = append(groups, buffer)
			}
			actual := readMultiPageIterator(t,
				NewMultiRepeatedPageIterator(
					NewRepeatedPageIterator(
						context.Background(), iter.NewSliceIterator(tc.rows), groups, 0, 1000),
					NewRepeatedPageIterator(
						context.Background(), iter.NewSliceIterator(tc.rows), groups, 1, 1000),
				),
			)
			if diff := cmp.Diff(tc.expected, actual, int64ParquetComparer()); diff != "" {
				t.Errorf("result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// readPageIterator reads all the values from the iterator and returns the result.
// Result are copied to avoid keeping reference between next calls.
func readPageIterator(t *testing.T, it iter.Iterator[*RepeatedRow[testRowGetter]]) []RepeatedRow[testRowGetter] {
	defer func() {
		require.NoError(t, it.Close())
	}()
	var result []RepeatedRow[testRowGetter]
	for it.Next() {
		current := RepeatedRow[testRowGetter]{
			Row:    it.At().Row,
			Values: make([]parquet.Value, len(it.At().Values)),
		}
		copy(current.Values, it.At().Values)
		if len(result) > 0 && current.Row.RowNumber() == result[len(result)-1].Row.RowNumber() {
			result[len(result)-1].Values = append(result[len(result)-1].Values, current.Values...)
			continue
		}

		result = append(result, current)
	}
	require.NoError(t, it.Err())
	return result
}

func readMultiPageIterator(t *testing.T, it iter.Iterator[*MultiRepeatedRow[testRowGetter]]) []MultiRepeatedRow[testRowGetter] {
	defer func() {
		require.NoError(t, it.Close())
	}()
	var result []MultiRepeatedRow[testRowGetter]
	for it.Next() {
		current := MultiRepeatedRow[testRowGetter]{
			Row:    it.At().Row,
			Values: make([][]parquet.Value, len(it.At().Values)),
		}
		for i, v := range it.At().Values {
			current.Values[i] = make([]parquet.Value, len(v))
			copy(current.Values[i], v)
		}
		if len(result) > 0 && current.Row.RowNumber() == result[len(result)-1].Row.RowNumber() {
			for i, v := range current.Values {
				result[len(result)-1].Values[i] = append(result[len(result)-1].Values[i], v...)
			}
			continue
		}

		result = append(result, current)
	}
	require.NoError(t, it.Err())
	return result
}

func int64ParquetComparer() cmp.Option {
	return cmp.Comparer(func(x, y parquet.Value) bool {
		return x.Int64() == y.Int64()
	})
}
