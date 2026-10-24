import type { CaseRow, Field, FieldRule } from '../types/api'

/** The most cases one run accepts (BRD-001 OD4, CASERUNNER_MAX_CASES default). */
export const MAX_CASES = 1000

/** A new row starts from the curl's own values (BRD-001 §8.2), with the expect left to fill. */
export function sampleRow(id: number, fields: Field[]): CaseRow {
  return { id, name: '', values: Object.fromEntries(fields.map((f) => [f.path, f.sampleValue])), expect: '' }
}

export interface CellProblem {
  path: string
  kind: 'required' | 'expect'
}

/** The problems that will make the backend BLOCK a row (D4, D15); the backend stays the source of truth. */
export function rowProblems(row: CaseRow, fields: Field[], rules: Record<string, FieldRule>): CellProblem[] {
  const problems: CellProblem[] = fields
    .filter((f) => (rules[f.path] ?? 'REQUIRED') === 'REQUIRED' && (row.values[f.path] ?? '') === '')
    .map((f) => ({ path: f.path, kind: 'required' }))
  if (row.expect.trim() === '') problems.push({ path: '', kind: 'expect' })
  return problems
}

/**
 * Keeps each row's values for the field paths that still exist after the
 * curl is analyzed again, and counts the non-empty values that would be lost.
 */
export function remapRows(rows: CaseRow[], fields: Field[]): { rows: CaseRow[]; lost: number } {
  const paths = new Set(fields.map((f) => f.path))
  let lost = 0
  const next = rows.map((row) => {
    for (const [path, value] of Object.entries(row.values)) {
      if (!paths.has(path) && value !== '') lost++
    }
    const values = Object.fromEntries(fields.map((f) => [f.path, row.values[f.path] ?? f.sampleValue]))
    return { ...row, values }
  })
  return { rows: next, lost }
}
