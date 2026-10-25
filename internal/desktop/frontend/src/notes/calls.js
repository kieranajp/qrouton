import { PRESENTER_NOTE } from "../lib/bridge/generated.js";
import { Call, call } from "../lib/wails.js";

export const presenterNote = () => call(Call.ByName(PRESENTER_NOTE));
