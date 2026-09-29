import { describe, expect, it } from "vitest";
import { validResetDeadline } from "./sandbox-reset";

describe("reset deadline input", () => {
  it("accepts the default and inclusive Core bounds without truncating invalid input", () => {
    for (const value of ["300", "3600", "86400"]) expect(validResetDeadline(value)).toBe(true);
    for (const value of ["", "299", "86401", "3600.5", "3e3", " 3600", "-300", "Infinity"]) expect(validResetDeadline(value)).toBe(false);
  });
});
