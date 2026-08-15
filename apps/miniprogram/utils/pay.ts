export type PayParams = {
  timeStamp: string;
  nonceStr: string;
  package: string;
  signType: string;
  paySign: string;
};

export type PrepayResp = {
  mock_payment?: boolean | string;
  zero_order?: boolean;
  prepay_id?: string;
  pay_params?: PayParams;
};

export function payAction(prepay: PrepayResp): "zero" | "mock" | "jsapi" | "invalid" {
  if (prepay.zero_order) return "zero";
  if (prepay.mock_payment === true || prepay.mock_payment === "true") return "mock";
  if (prepay.pay_params?.paySign && prepay.pay_params.package) return "jsapi";
  return "invalid";
}

export function paymentStatusLabel(status: string): "SUCCESS" | "FAILED" | "CONFIRMING" {
  if (status === "SUCCESS") return "SUCCESS";
  if (status === "CLOSED" || status === "PAYERROR") return "FAILED";
  return "CONFIRMING";
}

export function newIdempotencyKey(): string {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
