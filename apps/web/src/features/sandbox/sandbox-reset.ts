/** Keep the original field text until submission; decimals and empty values are not deadlines. */
export function validResetDeadline(value: string): boolean {
  return /^\d+$/.test(value) && Number(value) >= 300 && Number(value) <= 86400;
}
