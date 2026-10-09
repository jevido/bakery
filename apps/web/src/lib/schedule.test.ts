import { expect, test } from "bun:test";
import {
  buildCron,
  cronFieldsMessage,
  describeSchedule,
  parseCronToPreset,
  type Schedule,
} from "./schedule";

const pick = (s: Partial<Schedule>): Schedule => ({
  preset: "every_day",
  hour: "10",
  minute: "0",
  dayOfWeek: "1",
  dayOfMonth: "1",
  ...s,
});

test("buildCron builds each preset", () => {
  expect(buildCron(pick({ preset: "every_minute" }))).toBe("* * * * *");
  expect(buildCron(pick({ preset: "every_hour", minute: "15" }))).toBe("15 * * * *");
  expect(buildCron(pick({ preset: "every_day", hour: "9", minute: "30" }))).toBe("30 9 * * *");
  expect(buildCron(pick({ preset: "weekdays", hour: "8" }))).toBe("0 8 * * 1-5");
  expect(buildCron(pick({ preset: "weekly", hour: "9", dayOfWeek: "1" }))).toBe("0 9 * * 1");
  expect(buildCron(pick({ preset: "monthly", dayOfMonth: "15" }))).toBe("0 10 15 * *");
  expect(buildCron(pick({ preset: "custom" }))).toBe("");
});

test("parseCronToPreset round-trips every preset", () => {
  for (const s of [
    pick({ preset: "every_minute" }),
    pick({ preset: "every_hour", minute: "45" }),
    pick({ preset: "every_day", hour: "23", minute: "55" }),
    pick({ preset: "weekdays", hour: "7" }),
    pick({ preset: "weekly", hour: "9", dayOfWeek: "0" }),
    pick({ preset: "monthly", dayOfMonth: "31" }),
  ]) {
    expect(buildCron(parseCronToPreset(buildCron(s)))).toBe(buildCron(s));
    expect(parseCronToPreset(buildCron(s)).preset).toBe(s.preset);
  }
});

test("parseCronToPreset falls back to custom", () => {
  expect(parseCronToPreset("").preset).toBe("every_day");
  expect(parseCronToPreset("*/15 * * * *").preset).toBe("custom");
  expect(parseCronToPreset("7 9 * * 1").preset).toBe("custom");
  expect(parseCronToPreset("0 9 * *").preset).toBe("custom");
});

test("describeSchedule says it in words", () => {
  expect(describeSchedule("* * * * *")).toBe("Every minute");
  expect(describeSchedule("5 * * * *")).toBe("Every hour at :05");
  expect(describeSchedule("0 9 * * 1")).toBe("Every Mon at 9:00 AM");
  expect(describeSchedule("30 0 * * *")).toBe("Every day at 12:30 AM");
  expect(describeSchedule("0 13 * * 1-5")).toBe("Weekdays at 1:00 PM");
  expect(describeSchedule("0 10 2 * *")).toBe("Monthly on the 2nd at 10:00 AM");
  expect(describeSchedule("*/15 * * * *")).toBe("*/15 * * * *");
});

test("cronFieldsMessage asks for five fields", () => {
  expect(cronFieldsMessage("")).toBe("Enter a 5-field cron expression.");
  expect(cronFieldsMessage("0 9 *")).toBe("Use exactly 5 fields; this has 3.");
  expect(cronFieldsMessage("0 9 * * 1")).toBeNull();
});
