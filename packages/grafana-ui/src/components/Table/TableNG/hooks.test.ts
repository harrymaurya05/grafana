import { act, renderHook } from '@testing-library/react';

import { Field, FieldType } from '@grafana/data';

import { useFilteredRows, usePaginatedRows, useSortedRows, useFooterCalcs } from './hooks';
import { getColumnTypes } from './utils';

describe('TableNG hooks', () => {
  function setupData() {
    // Mock data for testing
    const fields: Field[] = [
      {
        name: 'name',
        type: FieldType.string,
        display: (v) => ({ text: v as string, numeric: NaN }),
        config: {},
        values: [],
      },
      {
        name: 'age',
        type: FieldType.number,
        display: (v) => ({ text: (v as number).toString(), numeric: v as number }),
        config: {},
        values: [],
      },
      {
        name: 'active',
        type: FieldType.boolean,
        display: (v) => ({ text: (v as boolean).toString(), numeric: NaN }),
        config: {},
        values: [],
      },
    ];

    const rows = [
      { name: 'Alice', age: 30, active: true, __depth: 0, __index: 0 },
      { name: 'Bob', age: 25, active: false, __depth: 0, __index: 1 },
      { name: 'Charlie', age: 35, active: true, __depth: 0, __index: 2 },
    ];

    return { fields, rows };
  }

  describe('useFilteredRows', () => {
    it('should correctly initialize with provided fields and rows', () => {
      const { fields, rows } = setupData();
      const { result } = renderHook(() => useFilteredRows(rows, fields, { hasNestedFrames: false }));
      expect(result.current.rows[0].name).toBe('Alice');
    });

    it('should apply filters correctly', () => {
      const { fields, rows } = setupData();
      const { result } = renderHook(() => useFilteredRows(rows, fields, { hasNestedFrames: false }));

      act(() => {
        result.current.setFilter({
          name: { filteredSet: new Set(['Alice']) },
        });
      });

      expect(result.current.rows.length).toBe(1);
      expect(result.current.rows[0].name).toBe('Alice');
    });

    it('should clear filters correctly', () => {
      const { fields, rows } = setupData();
      const { result } = renderHook(() => useFilteredRows(rows, fields, { hasNestedFrames: false }));

      act(() => {
        result.current.setFilter({
          name: { filteredSet: new Set(['Alice']) },
        });
      });

      expect(result.current.rows.length).toBe(1);

      act(() => {
        result.current.setFilter({});
      });

      expect(result.current.rows.length).toBe(3);
    });

    it.todo('should handle nested frames');
  });

  describe('useSortedRows', () => {
    it('should correctly set up the table with an initial sort', () => {
      const { fields, rows } = setupData();
      const columnTypes = getColumnTypes(fields);
      const { result } = renderHook(() =>
        useSortedRows(rows, fields, {
          columnTypes,
          hasNestedFrames: false,
          initialSortBy: [{ displayName: 'age', desc: false }],
        })
      );

      // Initial state checks
      expect(result.current.sortColumns).toEqual([{ columnKey: 'age', direction: 'ASC' }]);
      expect(result.current.rows[0].name).toBe('Bob');
    });

    it('should change the sort on setSortColumns', () => {
      const { fields, rows } = setupData();
      const columnTypes = getColumnTypes(fields);
      const { result } = renderHook(() =>
        useSortedRows(rows, fields, {
          columnTypes,
          hasNestedFrames: false,
          initialSortBy: [{ displayName: 'age', desc: false }],
        })
      );

      expect(result.current.rows[0].name).toBe('Bob');

      act(() => {
        result.current.setSortColumns([{ columnKey: 'age', direction: 'DESC' }]);
      });

      expect(result.current.rows[0].name).toBe('Charlie');

      act(() => {
        result.current.setSortColumns([{ columnKey: 'name', direction: 'ASC' }]);
      });

      expect(result.current.rows[0].name).toBe('Alice');
    });

    it.todo('should handle nested frames');
  });

  describe('usePaginatedRows', () => {
    it('should return defaults for pagination values when pagination is disabled', () => {
      const { rows } = setupData();
      const { result } = renderHook(() =>
        usePaginatedRows(rows, { rowHeight: 30, height: 300, width: 800, enabled: false })
      );

      expect(result.current.page).toBe(-1);
      expect(result.current.rowsPerPage).toBe(0);
      expect(result.current.pageRangeStart).toBe(1);
      expect(result.current.pageRangeEnd).toBe(3);
      expect(result.current.rows.length).toBe(3);
    });

    it('should handle pagination correctly', () => {
      // with the numbers provided here, we have 3 rows, with 2 rows per page, over 2 pages total.
      const { rows } = setupData();
      const { result } = renderHook(() =>
        usePaginatedRows(rows, {
          enabled: true,
          height: 60,
          width: 800,
          rowHeight: 10,
        })
      );

      expect(result.current.page).toBe(0);
      expect(result.current.rowsPerPage).toBe(2);
      expect(result.current.pageRangeStart).toBe(1);
      expect(result.current.pageRangeEnd).toBe(2);
      expect(result.current.rows.length).toBe(2);

      act(() => {
        result.current.setPage(1);
      });

      expect(result.current.page).toBe(1);
      expect(result.current.rowsPerPage).toBe(2);
      expect(result.current.pageRangeStart).toBe(3);
      expect(result.current.pageRangeEnd).toBe(3);
      expect(result.current.rows.length).toBe(1);
    });
  });

  describe('useFooterCalcs', () => {
    const rows = [
      { Field1: 1, Text: 'a', __depth: 0, __index: 0 },
      { Field1: 2, Text: 'b', __depth: 0, __index: 1 },
      { Field1: 3, Text: 'c', __depth: 0, __index: 2 },
      { Field2: 3, Text: 'd', __depth: 0, __index: 3 },
      { Field2: 10, Text: 'e', __depth: 0, __index: 4 },
    ];

    const numericField: Field = {
      name: 'Field1',
      type: FieldType.number,
      values: [1, 2, 3],
      config: {
        custom: {},
      },
      display: (value: unknown) => ({
        text: String(value),
        numeric: Number(value),
        color: undefined,
        prefix: undefined,
        suffix: undefined,
      }),
      state: {},
      getLinks: undefined,
    };

    const numericField2: Field = {
      name: 'Field2',
      type: FieldType.number,
      values: [3, 10],
      config: { custom: {} },
      display: (value: unknown) => ({
        text: String(value),
        numeric: Number(value),
        color: undefined,
        prefix: undefined,
        suffix: undefined,
      }),
      state: {},
      getLinks: undefined,
    };

    const textField: Field = {
      name: 'Text',
      type: FieldType.string,
      values: ['a', 'b', 'c'],
      config: { custom: {} },
      display: (value: unknown) => ({
        text: String(value),
        numeric: 0,
        color: undefined,
        prefix: undefined,
        suffix: undefined,
      }),
      state: {},
      getLinks: undefined,
    };

    it('should calculate sum for numeric fields', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, numericField], {
          enabled: true,
          footerOptions: { show: true, reducer: ['sum'] },
        })
      );

      expect(result.current).toEqual(['Total', '6']); // 1 + 2 + 3
    });

    it('should calculate mean for numeric fields', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, numericField], {
          enabled: true,
          footerOptions: { show: true, reducer: ['mean'] },
        })
      );

      expect(result.current).toEqual(['Mean', '2']); // (1 + 2 + 3) / 3
    });

    it('should return an empty string for non-numeric fields', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, textField], {
          enabled: true,
          footerOptions: { show: true, reducer: ['sum'] },
        })
      );

      expect(result.current).toEqual(['Total', '']);
    });

    it('should return empty array if no footerOptions are provided', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, textField], {
          enabled: true,
          footerOptions: undefined,
        })
      );

      expect(result.current).toEqual([]);
    });

    it('should return empty array when footer is disabled', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, textField], {
          enabled: false,
          footerOptions: { show: true, reducer: ['sum'] },
        })
      );

      expect(result.current).toEqual([]);
    });

    it('should return empty array when reducer is undefined', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, textField], {
          enabled: true,
          footerOptions: { show: true, reducer: undefined },
        })
      );

      expect(result.current).toEqual([]);
    });

    it('should return empty array when reducer is empty', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, textField], {
          enabled: true,
          footerOptions: { show: true, reducer: [] },
        })
      );

      expect(result.current).toEqual([]);
    });

    it('should return empty string if fields array doesnt include this field', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, numericField, numericField2], {
          enabled: true,
          footerOptions: { show: true, reducer: ['sum'], fields: ['Field2', 'Field3'] },
        })
      );

      expect(result.current).toEqual(['Total', '', '13']);
    });

    it('should return the calculation if fields array includes this field', () => {
      const { result } = renderHook(() =>
        useFooterCalcs(rows, [textField, numericField, numericField2], {
          enabled: true,
          footerOptions: { show: true, reducer: ['sum'], fields: ['Field1', 'Field2', 'Field3'] },
        })
      );

      expect(result.current).toEqual(['Total', '6', '13']);
    });
  });
});
