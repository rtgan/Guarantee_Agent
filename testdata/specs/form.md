# 表单全流程覆盖

## 前置条件
- 表单页面已可访问:{{BASE_URL}}/form.html

## 步骤
1. 打开 {{BASE_URL}}/form.html
2. 断言标题元素 "AutoQA Form Fixture" 可见
3. 在 Username 输入框填写 "alice"
4. 在 Favorite color 下拉框选择 Green
5. 点击 Submit 按钮
   - 预期: Form submitted successfully
6. 向下滚动页面
7. 验证 "You reached the bottom" 文字可见
8. 点击 Show After Wait 按钮
9. 等待 1000 毫秒
10. 验证 "Visible after wait" 文字可见
