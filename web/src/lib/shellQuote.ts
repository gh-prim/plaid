/**
 * Quotes one argument for a POSIX shell: single quotes, with each `'`
 * written as `'\''`. A folder name can then never run anything (F007 R9).
 */
export function shellQuote(arg: string): string {
  return `'${arg.replaceAll("'", `'\\''`)}'`
}

/** The fix for a folder the plaid user can't read (F007 R9). */
export function setfaclCommand(path: string): string {
  return `sudo setfacl -R -m u:plaid:rX ${shellQuote(path)}`
}

/** What to check for a library folder that went away (F007 R20). */
export function mountCommand(path: string): string {
  const p = shellQuote(path)
  return `findmnt ${p} || sudo mount ${p}`
}
