package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ResellerPlan holds the schema definition for the ResellerPlan entity.
//
// 与 VIPTier 刻意分开：VIP 是按消费挣来的等级，会升降会过期；分销商套餐是
// 买来的合作资格，带返还额度、域名配额与可转售分组。现在形状相似，但一个
// 随消费浮动、一个不动，混表迟早要拆。
//
// 删除策略：硬删除。等级是管理员维护的配置而非业务记录，下架用 enabled。
type ResellerPlan struct {
	ent.Schema
}

func (ResellerPlan) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "reseller_plans"},
	}
}

func (ResellerPlan) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ResellerPlan) Fields() []ent.Field {
	return []ent.Field{
		field.Int("level").
			Positive().
			Unique(),

		field.String("name").
			MaxLen(50).
			NotEmpty(),

		field.Float("price").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),

		// credit_rate: 购买价中返还为余额的比例。0.6 = 付 100 返 60。
		//
		// 用一次性返还而不是长期充值折扣：折扣对每一笔充值永久生效，还会与
		// 分组倍率、VIP 倍率连乘，账面上看不出来；返还有上限、看得见、能停。
		field.Float("credit_rate").
			SchemaType(map[string]string{dialect.Postgres: "decimal(6,4)"}).
			Default(0.5),

		// concurrency_bonus: 累加到 users.concurrency 之上，与 VIP 同为相加语义。
		//
		// 默认 0 且刻意给得保守：整池并发上限是「可用账号数 × 3」，账号被限流
		// 时可用数会掉到个位数。并发是当前最稀缺的资源。
		field.Int("concurrency_bonus").
			NonNegative().
			Default(0),

		field.Int("rpm_limit").
			NonNegative().
			Default(0),

		field.Int("max_domains").
			NonNegative().
			Default(1),

		field.Int("validity_days").
			Positive().
			Default(365),

		// allowed_group_ids: 可生成兑换码的分组白名单，空 = 不限制。
		//
		// 分销商挑我们定义好的分组来卖，而不是自己调费率：让他们调费率只会
		// 让他们自己的客户拿到更少 token，对他们的利润毫无帮助。
		field.JSON("allowed_group_ids", []int64{}).
			Default([]int64{}),

		field.Bool("enabled").
			Default(true),
	}
}

func (ResellerPlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "level"),
	}
}
