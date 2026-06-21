# 搜索过滤流程

## 前置条件
- 搜索页已可访问:{{BASE_URL}}/search.html

## 步骤
1. 打开 {{BASE_URL}}/search.html
2. 验证 "Banana" 文字可见
3. 在搜索框填写 "an"
   - 预期: Mango
4. 验证 "Mango" 文字可见
5. 清空搜索框并填写 "zzz"
   - 预期: No results
6. 验证 "No results" 文字可见
