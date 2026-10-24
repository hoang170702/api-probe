# Hướng dẫn sử dụng API Case Runner

API Case Runner giúp tester lấy một request API dưới dạng lệnh `curl` rồi chạy nó với nhiều bộ dữ liệu, không cần viết code. Hướng dẫn này nói về những gì phiên bản hiện tại làm được: **lấy token khi API cần, dán curl, kiểm tra request, và đặt rule cho từng field trong JSON body**. Nhập test case, chạy và xuất báo cáo sẽ có ở các bản tiếp theo ([BRD-001 §18](brd/BRD-001.vi.md#18-kế-hoạch-implement-từng-step-nhỏ)).

Bản tiếng Anh: [user-guide.md](user-guide.md).

## 1. Khởi động API Case Runner

API Case Runner là **một chương trình duy nhất**, phục vụ cả trang web lẫn API của nó; không phải bật FE và BE riêng.

**Lần đầu (hoặc sau khi kéo code mới về): build và chạy bằng một lệnh.** Trong thư mục `caserunner` của repo:

| Windows (PowerShell) | Linux / macOS / WSL |
|---|---|
| `powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Run` | `scripts/build.sh --run` |

Cửa sổ sẽ hiện lần lượt từng bước:

```text
==> frontend                      build trang web (lần đầu tải thêm các package của nó)
==> embed                         đưa trang web vào trong chương trình
==> backend                       biên dịch ra file chương trình
Built …/backend/bin/caserunner    file này lần sau chạy thẳng được
==> run (Ctrl+C to stop)          tool đang khởi động
… caserunner listening url=http://127.0.0.1:8080
```

Nếu API Case Runner đang chạy sẵn, script sẽ tắt nó trước (`==> stopping the running API Case Runner (PID …)`) rồi mới bật bản vừa build, nên không cần tự tắt trước khi build lại. Script không bao giờ tắt chương trình khác: nếu cổng đang bị chương trình khác dùng, nó báo tên chương trình đó và dừng lại.

Khi thấy dòng `listening`, mở **http://127.0.0.1:8080** trên trình duyệt. Giữ cửa sổ đó mở trong lúc dùng tool; nhấn **Ctrl+C** trong cửa sổ để dừng. Nếu script dừng ngay từ đầu với một dòng `error:`, nó sẽ nêu thứ còn thiếu (Go hoặc Node.js) và in sẵn lệnh để cài.

**Những lần sau: chỉ cần chạy file đã build**; khởi động trong một giây và không cần Go hay Node.js:

| Windows (PowerShell) | Linux / macOS / WSL |
|---|---|
| `.\backend\bin\caserunner.exe`, hoặc nhấp đúp vào file | `backend/bin/caserunner` |

Nếu developer đưa sẵn chương trình đã build (`caserunner.exe` trên Windows) thì bạn chỉ cần bước cuối này.

Tag ở góc trên bên phải trang hiện **BE OK** kèm phiên bản khi tool đã sẵn sàng. Nút **VI / EN** bên cạnh đổi ngôn ngữ của trang; lựa chọn được ghi nhớ.

| Muốn | Windows (PowerShell) | Linux / macOS / WSL |
|---|---|---|
| Dừng | `Ctrl+C` trong cửa sổ đang chạy, hoặc đóng cửa sổ | `Ctrl+C` trong terminal đang chạy |
| Chạy nền | `Start-Process -WindowStyle Hidden .\backend\bin\caserunner.exe`, dừng bằng `Stop-Process -Name caserunner` | `nohup backend/bin/caserunner > caserunner.log 2>&1 &`, dừng bằng `kill <pid>` |
| Dùng cổng khác, hoặc cho đồng nghiệp mở (mặc định chỉ máy của bạn truy cập được) | `$env:CASERUNNER_BIND = '0.0.0.0:9090'; .\backend\bin\caserunner.exe` | `CASERUNNER_BIND=0.0.0.0:9090 backend/bin/caserunner` |

Build cần Go 1.27+ và Node.js 20.19+ (xem [README](../README.md#quick-start) để biết cách cài).

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

## 6. Lấy token trước (tuỳ chọn)

Hầu hết API ngoài máy của developer đều cần token, thường lấy từ một API login. Thay vì tự copy token (và copy lại mỗi khi hết hạn), hãy để tool tự lấy:

1. Trong **0. Lấy token (tuỳ chọn)**, bấm **Thêm bước lấy token** rồi dán curl của API login (hoặc API cấp token), copy giống như ở mục 2. Bấm **Analyze**.
2. Ở **Giá trị cần bắt từ response**, biến đầu tiên đã có tên `token` và ô **Lấy từ field** để trống: khi đó tool tự tìm token, bằng cách tìm field tên `access_token`, `accessToken`, `token`, `id_token`, `jwt`, … ở mọi tầng. Nếu API của bạn để token ở chỗ khác, gõ đường dẫn tới field đó, mỗi tầng cách nhau bằng dấu chấm: với response `{"data":{"auth":{"value":"eyJ…"}}}` thì ghi `data.auth.value`; field nằm trong mảng thì thêm vị trí, ví dụ `data.items[0].id`.
3. Bấm **Chạy thử**. Mỗi bước hiện **Thành công** hoặc lý do lỗi, mã HTTP, giá trị bắt được (bị che; bật **Hiện giá trị nhạy cảm** để xem) và nơi tìm thấy, ví dụ *lấy từ data.accessToken*. Mở **Response body** để xem toàn bộ response.
4. Giữ công tắc **Gắn Authorization: Bearer {{token}} vào curl chính** ở trạng thái bật: khi chạy case, curl chính được gắn `Authorization: Bearer <token>` mới, thay cho cái bạn đã dán. Ở **2. Request**, dòng `Authorization` cũ bị gạch để cho biết điều đó.

![Một bước lấy token đã tự tìm thấy token](images/05-token-step-vi.png)

Khi chạy case, các bước lấy token chạy lại **trước mỗi case**, nên mỗi case dùng một token mới và các case chạy lần lượt. *(Phần chạy case có ở bản tiếp theo; hiện tại **Chạy thử** giúp kiểm tra việc lấy token.)*

**Flow nhiều bước** (login → OTP → …): thêm bước. Bước sau dùng được giá trị bước trước đã bắt bằng cách viết `{{name}}` ở bất kỳ đâu trong URL, header hay body, ví dụ `-H 'Authorization: Bearer {{token}}'` hoặc `"session":"{{sessionId}}"`. Curl chính cũng dùng `{{name}}` được như vậy; **2. Request** liệt kê các biến nó dùng và đánh dấu biến nào chưa có bước nào bắt.

| Kết quả bước | Cách xử lý |
|---|---|
| *Expected a 2xx response, got HTTP 401* | Bản thân việc login bị từ chối: kiểm tra tài khoản trong curl của bước. |
| *token: no token field found in the response* | Token có tên khác: mở **Response body**, tìm nó rồi gõ path vào. |
| *token: no value at data.x* | Path sai; đối chiếu lại với **Response body**. |
| *Host … is not in CASERUNNER_ALLOWED_HOSTS* | Tool được cấu hình chỉ gọi tới một số host; nhờ người vận hành thêm host này. |
| *Unknown variable {{x}}* | Chưa có bước nào trước đó bắt `x`; thêm capture hoặc sửa lại tên. |

## 7. Khi curl không dùng được

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

## 8. Nên biết

- API Case Runner không lưu gì xuống đĩa. Đóng hoặc tải lại trang là mất những gì đã nhập.
- Curl của bạn, kể cả token, chỉ được gửi tới server API Case Runner của chính bạn và không bao giờ bị ghi vào log.
- **Chạy thử** gửi request thật. Chỉ dùng API Case Runner với môi trường LOCAL, DEV hoặc UAT, không bao giờ với production. Người vận hành tool có thể giới hạn các host được gọi bằng `CASERUNNER_ALLOWED_HOSTS`, ví dụ `CASERUNNER_ALLOWED_HOSTS='localhost,*.uat.example.com'` ([BRD-001 §13](brd/BRD-001.vi.md#13-an-toàn--bảo-mật)).

## 9. Xử lý sự cố

| Hiện tượng | Cách xử lý |
|---|---|
| Tag đỏ **BE down** và banner *Backend unreachable* | Chương trình API Case Runner chưa chạy hoặc đã dừng; chạy lại rồi tải lại trang. |
| `scripts/build.sh` dừng với *the Windows Node.js is on PATH* hoặc *node was not found*, hoặc in ra `UNC paths are not supported` | Trong WSL, việc build cần Node.js cài bên trong Linux, không phải bản của Windows: cài bằng nvm (`nvm install 22`) như script hướng dẫn, mở terminal mới rồi build lại. |
| Trang báo *The frontend is not built into this binary* | Chương trình được build mà không có trang web; build lại bằng `scripts/build.sh`. |
| Báo `address already in use` (Linux) hoặc *Only one usage of each socket address is normally permitted* (Windows) khi khởi động | Chương trình khác đang dùng cổng 8080, thường là một API Case Runner đang chạy sẵn; dừng nó, hoặc chạy với cổng khác (`CASERUNNER_BIND=127.0.0.1:9090`) rồi mở cổng đó. |
| *port 8080 is used by … (PID …), not API Case Runner* khi build với `--run` / `-Run` | Chương trình khác đang dùng cổng đó; tắt nó, hoặc chọn cổng khác bằng `CASERUNNER_BIND` (xem mục 1). |
| *API Case Runner is running (PID …), so the build cannot replace caserunner.exe* (Windows) | Build với `-Run` để khởi động lại, hoặc tắt nó trước. |
| PowerShell báo *running scripts is disabled on this system* | Chạy build bằng `powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1`. |
| `build.ps1` dừng với *Node.js v… is too old* hoặc *was not found on PATH* | Cài thứ nó nêu bằng lệnh `winget` nó in ra, mở cửa sổ PowerShell mới rồi build lại. |
| Service dừng ngay kèm thông báo nêu tên một biến `CASERUNNER_` | Biến môi trường đó có giá trị sai; sửa lại hoặc bỏ nó đi ([BRD-001 §20](brd/BRD-001.vi.md#20-cấu-hình-env)). |
