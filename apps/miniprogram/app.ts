App({
  globalData: {
    token: "",
    scene: "PICKUP" as "PICKUP" | "DINE_IN",
    tableToken: "",
    tableNo: "",
    sceneLocked: false,
  },
  onLaunch(options: WechatMiniprogram.App.LaunchShowOption) {
    const scene = options.query?.scene || "";
    if (typeof scene === "string" && scene) {
      this.globalData.scene = "DINE_IN";
      this.globalData.tableToken = scene;
      this.globalData.sceneLocked = true;
    } else {
      this.globalData.scene = "PICKUP";
      this.globalData.tableToken = "";
      this.globalData.tableNo = "";
      this.globalData.sceneLocked = false;
    }
  },
});
