/**
 * With two or more profiles, the picker shows at the first route of each
 * browser tab session (F008-R2); this flag, in sessionStorage, says the
 * tab already chose.
 */
const pickedKey = 'plaid.profilePicked'

export function profilePicked(): boolean {
  try {
    return sessionStorage.getItem(pickedKey) === '1'
  } catch {
    return true // No storage: never ask twice in a row.
  }
}

export function markProfilePicked() {
  try {
    sessionStorage.setItem(pickedKey, '1')
  } catch {
    // Not remembered; the picker shows again on the next tab.
  }
}
