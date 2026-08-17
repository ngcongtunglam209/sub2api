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

// ResellerDomain holds the schema definition for the ResellerDomain entity.
//
// 分销商把自己的域名 CNAME 到本站，其客户只看到分销商的域名。这张表既是
// Caddy on_demand_tls 的签发白名单，也是按 Host 的请求准入名单——两者必须
// 是同一份数据，否则停用一个分销商后，已签发的证书还能让他白用近 90 天。
//
// 删除策略：硬删除。域名是运营配置而非业务记录，停用用 status，删除即彻底移除。
type ResellerDomain struct {
	ent.Schema
}

func (ResellerDomain) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "reseller_domains"},
	}
}

func (ResellerDomain) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ResellerDomain) Fields() []ent.Field {
	return []ent.Field{
		// domain: 完整域名，如 api.brand.com。
		//
		// 253 是 DNS 名称长度上限。统一小写存储，查询前也小写化：Host 头的
		// 大小写由客户端决定，不能指望它规范。
		field.String("domain").
			MaxLen(253).
			NotEmpty().
			Unique(),

		// user_id: 归属的分销商用户。
		//
		// 不建外键也不设 edge：域名是运营配置，用户被删除时由后台清理，
		// 让级联删除悄悄带走一个还在解析的域名反而更危险。
		field.Int64("user_id"),

		// status: 只有 active 才签发证书并放行流量。
		field.String("status").
			MaxLen(20).
			Default("active"),

		field.String("notes").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (ResellerDomain) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
