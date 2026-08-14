package print

import (
	"fmt"
	"strings"
)

type Ticket struct {
	StoreName     string
	PickupNumber  string
	TableNo       string
	PickupType    string
	ScheduledFor  string
	Remark        string
	Items         []TicketItem
	PayableCents  int64
	IsMock        bool
}

type TicketItem struct {
	Name string
	Qty  int
}

func RenderOrder(t Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", t.StoreName)
	if t.PickupType == "SCHEDULED" && t.ScheduledFor != "" {
		fmt.Fprintf(&b, "预约自取 %s\n", t.ScheduledFor)
	}
	if t.PickupNumber != "" {
		fmt.Fprintf(&b, "取餐号 %s\n", t.PickupNumber)
	}
	if t.TableNo != "" {
		fmt.Fprintf(&b, "桌号 %s\n", t.TableNo)
	} else if t.PickupType != "SCHEDULED" {
		b.WriteString("到店自取\n")
	}
	for _, it := range t.Items {
		fmt.Fprintf(&b, "%s x%d\n", it.Name, it.Qty)
	}
	if t.Remark != "" {
		fmt.Fprintf(&b, "备注 %s\n", t.Remark)
	}
	fmt.Fprintf(&b, "应付 ¥%.2f\n", float64(t.PayableCents)/100)
	if t.IsMock {
		b.WriteString("【测试单】\n")
	}
	return b.String()
}
