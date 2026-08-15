export function inspectMerchantCert(pem: string): { ok: boolean; reason: string } {
  const t = pem.trim();
  if (!t) return { ok: false, reason: "空证书" };
  if (!t.includes("BEGIN CERTIFICATE") && !t.includes("BEGIN PRIVATE KEY") && !t.includes("BEGIN RSA PRIVATE KEY")) {
    return { ok: false, reason: "不是 PEM" };
  }
  if (t.includes("BEGIN CERTIFICATE") && (t.includes("BEGIN PRIVATE KEY") || t.includes("BEGIN RSA PRIVATE KEY"))) {
    return { ok: true, reason: "证书与私钥均已粘贴，将在服务端校验匹配" };
  }
  if (t.includes("BEGIN CERTIFICATE")) return { ok: true, reason: "已识别商户证书，不会保存证书文件本身" };
  return { ok: true, reason: "已识别私钥 PEM" };
}

export function playBeep(volume = 0.4) {
  const ctx = new AudioContext();
  const osc = ctx.createOscillator();
  const gain = ctx.createGain();
  osc.type = "sine";
  osc.frequency.value = 880;
  gain.gain.value = volume;
  osc.connect(gain);
  gain.connect(ctx.destination);
  osc.start();
  osc.stop(ctx.currentTime + 0.18);
}
