export const SOURCE_FACTS = [
  { title: 'MacCMS XML / JSON', body: 'type 0 与 type 1。按响应体判断格式，声明 XML 的接口返回 JSON 也能读。' },
  { title: '常见的 drpy 与 XBPQ', body: '.js 规则、csp_XBPQ，以及和它同字段的 XYQHiker。不是完整的 drpy2 模板引擎。' },
  { title: 'jar、dex、Python 不执行', body: '其它 csp、.jar、DEX、.py 在片源列表里标成不支持，不会加载。' },
  { title: '订阅可以套几层', body: '纯 JSON、带注释的宽松 JSON、Base64、前缀加 Base64、gzip；urls / storeHouse 逐层打开。' },
] as const;

export const CONFIG_CODE = `{
  "sites": [
    {
      "key": "home",
      "name": "家庭片库",
      "type": 1,
      "api": "https://example.com/api.php/provide/vod"
    }
  ]
}`;
