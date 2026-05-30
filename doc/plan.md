
目标 完善 gopro的go 项目 。使用 qjs 作为js引擎，将 Kotlin 项目的 json 文件 中的 js 在java中执行，改变用go实现相关功能。

qj使用示例 在 gopro/tool/test——js2.go 和 gopro/tool/test.md
读取 gopro/doc/ding.md 和 gopro/doc/yue.md 是相关规则
补充文件 app/src/main/assets/web/help/md/ruleHelp.md 和 app/src/main/assets/web/help/md/jsHelp.md

核心的 kotlin 文件是 app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt。

用于测试的json文件位于 gopro/doc/example.json

主要实现的功能有
- 搜书，根据关键词，通过读取JSON中的书源，并发搜索书籍，返回搜索结果
- 根据选择的书，获取书籍信息和目录
- 根据目录，获取章节内容
