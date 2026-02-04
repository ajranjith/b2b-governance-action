import { withAdminEnvelope } from "@b2b/application";

const API_IDS = {} as any;
const REF_IDS = {} as any;
const policy = { check: (_id: string) => true };

export const GET = withAdminEnvelope(REF_IDS, API_IDS, async () => {
  policy.check("POL-A-00");
  return { ok: true };
});

export function boot() {
  return "ok";
}
