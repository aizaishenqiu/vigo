// Package common 用户自定义公共方法包
// 
// 说明：
// 1. 本目录用于存放用户自定义的公共方法
// 2. 所有在此目录中定义的方法都可以在全局任意位置调用
// 3. 用户可以根据自己的业务需求，在此添加各种工具函数
//
// 使用示例：
//   import "vigo/app/common"
//   
//   // 调用自定义方法
//   result := common.MyCustomFunction(param1, param2)
//
// 最佳实践：
// 1. 按功能模块组织文件，如：string.go, array.go, file.go 等
// 2. 所有函数首字母大写，以便在其他包中访问
// 3. 为每个函数添加详细的注释说明
// 4. 避免与框架 helper 包中的函数重名
//
// 示例文件：
// - common_example.go: 使用示例（可以删除）

package common

// 在这里添加你的自定义公共方法
// 例如：
// func MyCustomFunction(param1 string, param2 int) string {
//     // 你的实现
//     return ""
// }
