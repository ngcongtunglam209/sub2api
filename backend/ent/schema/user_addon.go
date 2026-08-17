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

// UserAddon holds the schema definition for the UserAddon entity.
//
// 用户用自己的余额买来的并发 / RPM 额度。与 VIP 等级、分销商套餐同为"加数"：
// 它们各自是单独掏钱买的东西，谁也不吞并谁。
//
// 每个 (user_id, kind) 只有一行，重复购买续期而非新增行——理由见
// migrations/228_user_addons.sql 的表注释，简言之：鉴权热路径上一行比一次
// SUM 便宜，上限按总量卡也只有一行时最不容易判错。
//
// 删除策略：硬删除。过期的额度不是业务记录，扫描任务直接删。
type UserAddon struct {
	ent.Schema
}

func (UserAddon) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_addons"},
	}
}

func (UserAddon) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserAddon) Fields() []ent.Field {
	return []ent.Field{
		// user_id: 归属用户。
		//
		// 不建外键也不设 edge，与 reseller_domains 同理：额度是计费配置，
		// 用户删除时由后台清理比级联静默带走更安全。
		field.Int64("user_id"),

		// kind: concurrency | rpm。
		//
		// 用字符串而不是枚举表：新增一种可加购资源只该是一行常量，不该是
		// 一次迁移。取值由 service 层的 AddonKind 校验。
		field.String("kind").
			MaxLen(20).
			NotEmpty(),

		// amount: 当前持有量。并发为槽位数，RPM 为每分钟请求数。
		field.Int("amount").
			NonNegative().
			Default(0),

		// expires_at: 到期时间。
		//
		// 读取时即判定失效（见 AddonService.ResolveActiveAddons），扫描任务
		// 只负责清理。扫描停了顶多留下垃圾行，不会继续白送额度。
		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UserAddon) Indexes() []ent.Index {
	return []ent.Index{
		// Unique：一行一 (user, kind) 是这张表的不变量，靠约束守住而不是靠
		// 每个写入方自觉。
		index.Fields("user_id", "kind").Unique(),
		index.Fields("expires_at"),
	}
}
