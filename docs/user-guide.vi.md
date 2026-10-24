# Hướng dẫn sử dụng API Case Runner

API Case Runner giúp tester lấy một request API dưới dạng lệnh `curl` rồi chạy nó với nhiều bộ dữ liệu, không cần viết code. Hướng dẫn này nói về những gì phiên bản hiện tại làm được: **dán curl, kiểm tra request, và đặt rule cho từng field trong JSON body**. Nhập test case, chạy và xuất báo cáo sẽ có ở các bản tiếp theo ([BRD-001 §18](brd/BRD-001.vi.md#18-kế-hoạch-implement-từng-step-nhỏ)).

Bản tiếng Anh: [user-guide.md](user-guide.md).

## 1. Khởi động API Case Runner

API Case Runner là một chương trình duy nhất, phục vụ cả trang web lẫn API của nó.

```sh
cd caserunner
scripts/build.sh          # chạy một lần, hoặc sau khi kéo code mới về
backend/bin/caserunner
```

Sau đó mở **http://127.0.0.1:8080** trên trình duyệt. Tag ở góc trên bên phải hiện **BE OK** kèm phiên bản khi tool đã sẵn sàng.

| Muốn | Làm |
|---|---|
| Chạy tiếp sau khi đóng terminal | `nohup backend/bin/caserunner > caserunner.log 2>&1 &`, khi cần dừng thì `kill <pid>` |
| Dùng cổng khác, hoặc cho đồng nghiệp mở | `CASERUNNER_BIND=0.0.0.0:9090 backend/bin/caserunner` (mặc định chỉ máy của bạn truy cập được) |
| Dừng | `Ctrl+C` trong terminal đang chạy nó |

Build cần Go 1.27+ và Node.js 20.19+; nếu developer đưa sẵn file `caserunner` đã build thì chỉ cần chạy nó.

## 2. Copy request dưới dạng curl

| Từ | Cách làm |
|---|---|
| Chrome / Edge DevTools | Tab Network → chuột phải vào request → **Copy** → **Copy as cURL (bash)** hoặc **Copy as cURL (cmd)** |
| Firefox DevTools | Tab Network → chuột phải vào request → **Copy Value** → **Copy as cURL** |
| Postman | Mở request → **Code** (`</>`) → **cURL** → copy |

Cả kiểu bash lẫn kiểu Windows cmd đều dùng được; không cần sửa lại chuỗi.

## 3. Dán và phân tích

Dán lệnh vào **1. Curl** rồi bấm **Analyze** (hoặc nhấn **Ctrl+Enter**).

![Phần Curl với lệnh đã dán](images/01-paste-curl.png)

## 4. Kiểm tra request

**2. Request** cho thấy API Case Runner đã hiểu request thế nào:

- **Request**: method và URL (biểu tượng copy để chép URL).
- **Body**: `JSON`, `No body` hoặc `Unsupported`.
- **Cảnh báo**: những gì tool đã bỏ qua hoặc thay đổi, ví dụ *"Ignored --compressed"*. Nên đọc qua; cảnh báo không bao giờ chặn bạn, nhưng giải thích chỗ khác biệt so với khi tự chạy curl.
- **Headers**: mọi header theo đúng thứ tự. Giá trị của `Authorization`, `Cookie`, `X-Api-Key` và header có chứa `token` hay `secret` bị che (6 ký tự đầu + `***`). Bật **Show sensitive values** để xem đầy đủ.

![Tóm tắt request và field rules sau khi Analyze](images/02-request-and-fields.png)

## 5. Đặt rule cho field

**3. Field rules** liệt kê mọi field của JSON body, theo đúng thứ tự trong body:

| Cột | Ý nghĩa |
|---|---|
| **Field path** | Vị trí của field: `data.account.id` là `id` nằm trong `account` nằm trong `data`; `items[0].sku` là `sku` trong phần tử đầu tiên của mảng `items`. Key có dấu chấm hay dấu ngoặc vuông được viết trong ngoặc kép, ví dụ `["key.with.dot"]`. |
| **Type** | `string`, `number`, `boolean`, `null`, hoặc `object` / `array` cho `{}` / `[]` rỗng. |
| **Sample value** | Giá trị trong curl của bạn, viết theo đúng cách bạn sẽ gõ vào test case: `<EMPTY>` là chuỗi rỗng `""`, `<NULL>` là `null`. Số được giữ đủ mọi chữ số, ví dụ số tài khoản 20 chữ số. |
| **Rule** | **Required** (mặc định) hoặc **Unrequired**. |

Ý nghĩa của rule khi chạy case:

- **Required**: mọi case phải điền giá trị cho field này. Case nào để trống sẽ bị **BLOCKED** và không bao giờ được gửi đi.
- **Unrequired**: case được phép để trống, và khi đó field **bị bỏ hẳn khỏi request**. Dùng cách này để thử API xử lý thế nào khi thiếu field.

**All Required** và **All Unrequired** đặt mọi rule cùng lúc; bộ đếm phía trên bảng cho biết mỗi loại có bao nhiêu. Nếu bạn sửa curl rồi Analyze lại, các field vẫn còn sẽ giữ rule bạn đã chọn.

## 6. Khi curl không dùng được

Thông báo dưới ô curl cho biết cần sửa gì. Vị trí được đếm theo ký tự từ đầu chuỗi.

![Curl đọc body từ file](images/03-parse-error.png)

| Thông báo (phần đầu) | Cách xử lý |
|---|---|
| *Missing closing ' for the quote opened at position N* | Chuỗi copy bị cắt hoặc thiếu dấu nháy; copy lại request. |
| *A curl command must start with curl* | Chỉ dán lệnh, bắt đầu từ chữ `curl`. |
| *No URL found* / *Found a second URL* | Giữ đúng một URL trong lệnh. |
| *-d @file reads the body from a file* | Thay `@file` bằng chính nội dung body. |
| *-G is not supported* / *-T is not supported* | Các flag này đổi request theo cách API Case Runner không tái tạo được; bỏ chúng đi và đưa dữ liệu vào body. |
| *Only http and https URLs are supported* | Dùng URL `http://` hoặc `https://`. |
| *The JSON body cannot be split into fields: duplicate key* | Body lặp một key trong cùng object; xoá key bị trùng. |

Request có body dạng form hoặc upload file (`-F`, `--data-urlencode`), hoặc body không phải JSON, vẫn được phân tích nhưng hiện **v1 supports JSON bodies only** và không chạy được:

![Body dạng form không được hỗ trợ](images/04-unsupported-body.png)

Các cảnh báo thường gặp, không cần làm gì:

| Cảnh báo | Ý nghĩa |
|---|---|
| *Ignored --compressed*, *Ignored -s*, … | Tuỳ chọn về output và transport; không đổi request. |
| *Ignored -L: redirects are never followed* | Redirect (3xx) sẽ được báo làm kết quả thay vì bị follow. |
| *Ignored --max-time: the run's timeout applies* | Timeout được đặt theo từng run trong API Case Runner. |
| *No Content-Type header* | API có thể không đọc body như JSON; thêm `-H 'Content-Type: application/json'` nếu cần. |
| *The URL has no scheme; using http://…* | Thêm `https://` nếu API cần. |

## 7. Nên biết

- API Case Runner không lưu gì xuống đĩa. Đóng hoặc tải lại trang là mất những gì đã nhập.
- Curl của bạn, kể cả token, chỉ được gửi tới server API Case Runner của chính bạn và không bao giờ bị ghi vào log.
- Chỉ dùng API Case Runner với môi trường LOCAL, DEV hoặc UAT, không bao giờ với production. Allowlist host để bắt buộc điều này đang được lên kế hoạch ([BRD-001 §13](brd/BRD-001.vi.md#13-an-toàn--bảo-mật)).

## 8. Xử lý sự cố

| Hiện tượng | Cách xử lý |
|---|---|
| Tag đỏ **BE down** và banner *Backend unreachable* | Chương trình API Case Runner chưa chạy hoặc đã dừng; chạy lại rồi tải lại trang. |
| `scripts/build.sh` dừng với *the Windows Node.js is on PATH* hoặc *node was not found*, hoặc in ra `UNC paths are not supported` | Trong WSL, việc build cần Node.js cài bên trong Linux, không phải bản của Windows: cài bằng nvm (`nvm install 22`) như script hướng dẫn, mở terminal mới rồi build lại. |
| Trang báo *The frontend is not built into this binary* | Chương trình được build mà không có trang web; build lại bằng `scripts/build.sh`. |
| Báo `address already in use` khi khởi động | Chương trình khác đang dùng cổng 8080; chạy với `CASERUNNER_BIND=127.0.0.1:9090` và mở cổng đó. |
| Service dừng ngay kèm thông báo nêu tên một biến `CASERUNNER_` | Biến môi trường đó có giá trị sai; sửa lại hoặc bỏ nó đi ([BRD-001 §20](brd/BRD-001.vi.md#20-cấu-hình-env)). |
