import { withDealerEnvelope } from "@b2b/application";

const API_IDS = {} as any;
const REF_IDS = {} as any;
const policy = { check: (_id: string) => true };

export const GET = withDealerEnvelope(REF_IDS, API_IDS, async () => {
  policy.check("POL-D-00");
  return { ok: true };
});
