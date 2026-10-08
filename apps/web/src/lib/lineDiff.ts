// Paperclip's line diff (ui/src/lib/line-diff.ts; MIT, see NOTICE): the
// longest common subsequence of two texts' lines, as rows of unchanged,
// removed and added lines with their old and new line numbers.

export type DiffRowKind = 'context' | 'removed' | 'added'

export type DiffRow = {
  kind: DiffRowKind
  oldLineNumber: number | null
  newLineNumber: number | null
  text: string
}

export function buildLineDiff(oldText: string, newText: string): DiffRow[] {
  const oldLines = oldText.split('\n')
  const newLines = newText.split('\n')
  const oldCount = oldLines.length
  const newCount = newLines.length
  const dp = Array.from({ length: oldCount + 1 }, () => Array<number>(newCount + 1).fill(0))

  for (let i = oldCount - 1; i >= 0; i--) {
    for (let j = newCount - 1; j >= 0; j--) {
      dp[i][j] = oldLines[i] === newLines[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1])
    }
  }

  const rows: DiffRow[] = []
  let i = 0
  let j = 0
  while (i < oldCount && j < newCount) {
    if (oldLines[i] === newLines[j]) {
      rows.push({ kind: 'context', oldLineNumber: i + 1, newLineNumber: j + 1, text: oldLines[i] })
      i++
      j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      rows.push({ kind: 'removed', oldLineNumber: i + 1, newLineNumber: null, text: oldLines[i] })
      i++
    } else {
      rows.push({ kind: 'added', oldLineNumber: null, newLineNumber: j + 1, text: newLines[j] })
      j++
    }
  }
  for (; i < oldCount; i++) rows.push({ kind: 'removed', oldLineNumber: i + 1, newLineNumber: null, text: oldLines[i] })
  for (; j < newCount; j++) rows.push({ kind: 'added', oldLineNumber: null, newLineNumber: j + 1, text: newLines[j] })
  return rows
}
