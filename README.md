# kmerge

[**English**](./README-en.md) | **简体中文**

合并 kubernetes secret 数据，根据以下注解

- kmerge.io/primary 该配置表明其他 secret 会合并到该资源中，且只合并该资源有 key 的内容
- kmerge.io/name 跨命名空间级别，相同名称会合并
- kmerge.io/type 支持合并内容格式，支持配置 text(default), json, yaml
- namespace.kmerge.io/from 合并资源的命名空间指定，若未指定，则是全部命名空间
