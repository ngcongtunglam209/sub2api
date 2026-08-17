package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// VIPTier holds the schema definition for the VIPTier entity.
//
// 删除策略：硬删除
// 与 SubscriptionPlan 同理，等级是管理员维护的配置而非业务记录：下架用
// enabled，删除表示彻底移除。用户身上留存的是 vip_tier_id，等级被删后按
// 无等级处理。
type VIPTier struct {
	ent.Schema
}

func (VIPTier) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "vip_tiers"},
	}
}

func (VIPTier) Fields() []ent.Field {
	return []ent.Field{
		// 等级序号，1 起算。VIP0 不建行：它是“没有等级”的展示名，任何
		// 用户没匹配到等级时就处于 VIP0，因此没有可配置项。
		field.Int("level").
			Positive().
			Unique(),
		field.String("name").
			MaxLen(50).
			NotEmpty(),
		// 达到本级所需的累计消费（美元）。与 payment_orders.amount 同口径，
		// 该字段是全站统一的美元计价额（见 service/payment_order.go 的
		// createOrderInTx 注释）。
		field.Float("min_spend_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		// 计费倍率，乘在分组倍率之上。0.95 表示同样 token 只扣 95% 费用。
		// 允许 0，与账号倍率保持一致的语义（该等级不计费）。
		field.Float("rate_multiplier").
			SchemaType(map[string]string{dialect.Postgres: "decimal(6,4)"}).
			Default(1),
		// 该等级额外赠送的并发数，累加到 users.concurrency 之上（而非覆盖）。
		// users.concurrency 是可售卖的加购项，若此处取二者较大值，VIP 用户
		// 买的并发会被等级数字吞掉、等于白付钱，因此改为相加。
		// 注意：Positive() 意味着最小赠送为 1，没有“不送并发”的等级。
		field.Int("concurrency").
			Positive().
			Default(5),
		// 该等级额外赠送的 RPM，累加到 users.rpm_limit 之上。0 表示不赠送。
		//
		// 与 concurrency 不同，这里不加 Positive()：不送 RPM 是个合理档位，
		// 而"不送并发"在上面被 Positive() 排除了。
		field.Int("rpm_limit").
			NonNegative().
			Default(0),
		// 免除上限的两个开关。
		//
		// 不复用 0 当哨兵：users.rpm_limit 里 0 确实表示不限量，但本表的
		// rpm_limit 是加数，加 0 只能表示"不加"。一列无法同时表达"不加"与
		// "无上限"，混用会把最便宜的等级变成最好的等级。concurrency 更直接：
		// Positive() 让它根本存不下 0。
		//
		// 生效顺序见 api_key_auth_cache_impl.go：必须在分销商套餐与加购项
		// 叠加**之后**才清上限，否则"无上限"会被后面的加数覆盖成有限值。
		field.Bool("unlimited_rpm").
			Default(false),
		field.Bool("unlimited_concurrency").
			Default(false),
		// 等级有效期：每笔订单完成后从当天起顺延这么多天。
		field.Int("grace_days").
			Positive().
			Default(60),
		field.String("badge_color").
			MaxLen(20).
			Default(""),
		field.Bool("enabled").
			Default(true),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (VIPTier) Indexes() []ent.Index {
	return []ent.Index{
		// 定级查询按门槛倒序找第一条满足的等级。
		index.Fields("enabled", "min_spend_usd"),
	}
}
