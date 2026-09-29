import { homedir } from "node:os";
import { isAbsolute, join } from "node:path";

const home = process.env.OAC_DEV_HOME || join(homedir(), ".oac");
if (!isAbsolute(home)) throw new Error("OAC_DEV_HOME must be absolute.");
export const buildDirectory = join(home, "build", "parsar-example");
