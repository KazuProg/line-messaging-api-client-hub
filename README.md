# Webhook Hub

LINE Messaging API の Webhook を 1 回受信し、登録済みの複数転送先（子 Webhook）へ HTTP POST で転送するハブです。LINE 公式の Webhook URL に本ハブの `/callback` を指定し、本ハブ経由で自前の複数サービスへイベントを配信できます。

## 動作の流れ

1. LINE プラットフォームが、設定した Webhook URL（本ハブの `POST /callback`）へイベントを送信する。
2. 本ハブが `x-line-signature` と環境変数 `LINE_CHANNEL_SECRET` で署名を検証する。検証失敗時は `401 Unauthorized` を返す。
3. 検証成功後、登録済みの転送先（clients）へ、受信した Body をそのまま `Content-Type: application/json` で POST する。
4. **required** が `true` の転送先のいずれかが失敗（接続失敗または 2xx 以外）した場合、LINE へは **500 を返し、LINE 側で再送の対象になる**。
5. **required** が `false` の転送先は非同期で転送され、失敗しても LINE への応答には影響しない（200 のまま）。

## 必要要件（利用側）

- Docker または Docker Compose で本イメージを実行できる環境。

## 環境変数

| 変数名 | 必須 | 説明 |
|--------|------|------|
| `LINE_CHANNEL_SECRET` | はい | LINE チャネルの「チャネルシークレット」。Webhook 署名検証に使用します。未設定の場合は署名検証が常に失敗し、`/callback` は 401 を返します。 |
| `PORT` | いいえ | 待ち受けポート。省略時は `8080`。 |
| `CLIENTS_FILE` | いいえ | 転送先一覧の永続化先ファイルパス。省略時は `/data/clients.json`。コンテナ内のパスを指定します。 |

## 起動方法（Docker）

```bash
export LINE_CHANNEL_SECRET=your_channel_secret
docker compose up --build
```

起動後、デフォルトでは `http://localhost:8080` で待ち受けます。

## 注意: 公開するパスは `/callback` のみにすること

リバースプロキシやロードバランサで本ハブを公開する場合、**インターネット向けに公開するのは `/callback` だけ**にしてください。

- **`/callback`** … LINE プラットフォームからアクセスされるため、**グローバルに公開**する。
- **`/clients`** … 転送先の登録・削除・一覧取得に使う。**内部（管理用ネットワークやローカル）からのみアクセスできるように**し、インターネットからは遮断すること。
- **`/health`** … ヘルスチェック用。必要に応じてロードバランサなど内部からのみアクセス可能にし、外部公開は不要。

`/clients` を外部に露出すると、第三者が転送先の追加・削除を行えるため、意図しない経路へ Webhook が流れるおそれがあります。

## エンドポイント一覧

| メソッド | パス | 用途 |
|----------|------|------|
| POST | `/callback` | LINE Webhook 受信（LINE 公式からここへ送信する URL として設定する） |
| GET | `/health` | ヘルスチェック。200 で `{"status":"ok"}` を返す。 |
| GET | `/clients` | 登録済み転送先一覧の取得 |
| POST | `/clients` | 転送先の登録 |
| DELETE | `/clients` | 転送先の削除 |

---

## LINE Webhook 受信 `POST /callback`

- **利用者向けの役割**: LINE の「Webhook URL」に `https://あなたのドメイン/callback` を設定して使用します。
- **リクエスト**: LINE から送られるものと同じ（Body は JSON、ヘッダに `x-line-signature` が付与される）。
- **レスポンス**:
  - 署名検証失敗: `401 Unauthorized`
  - 転送先が 1 件も登録されていない: `503 Service Unavailable`（本文 `no webhook clients registered`）
  - **required** の転送先で配信失敗: `500 Internal Server Error`
  - 上記以外で成功: `200 OK`

本ハブは受信 Body をそのまま各転送先へ POST するだけなので、転送先側は LINE の Webhook ペイロード形式に従って処理してください。

---

## ヘルスチェック `GET /health`

- **用途**: ロードバランサやオーケストレータからの生存確認。
- **レスポンス**: `200 OK`、`Content-Type: application/json`、本文 `{"status":"ok"}`。
- メソッドは問いません（GET 以外でも 200 を返します）。

---

## 転送先一覧の取得 `GET /clients`

登録済みの転送先（Webhook URL と required フラグ）の一覧を JSON で返します。

- **レスポンス**: `200 OK`、`Content-Type: application/json`
- **本文例**:

```json
[
  {
    "webhook_url": "https://example.com/webhook",
    "required": true
  },
  {
    "webhook_url": "https://another.example/events",
    "required": false
  }
]
```

転送先が 0 件の場合は空配列 `[]` を返します。

---

## 転送先の登録 `POST /clients`

新しい転送先を 1 件追加します。同一の `webhook_url` が既に存在する場合は、`required` の値だけ更新されます。

- **リクエスト**
  - **Content-Type**: `application/json`
  - **Body**:
    - `webhook_url`（必須）: 転送先の URL。本ハブがここへ LINE の Webhook Body をそのまま POST します。
    - `required`（任意、既定値 `false`）: `true` にすると、この転送先への配信に失敗した場合に LINE への応答が 5xx になり、LINE が再送する対象にします。必須の転送先だけ確実に届けたい場合に `true` にします。

- **リクエスト例**:

```bash
curl -s -X POST http://localhost:8080/clients \
  -H "Content-Type: application/json" \
  -d '{"webhook_url":"https://example.com/webhook","required":true}'
```

- **レスポンス**:
  - 成功時: `200 OK`、本文に登録内容 `{"webhook_url":"...","required":true|false}` を返す。
  - `webhook_url` が空または Body が不正: `400 Bad Request`（本文は JSON で `{"error":"..."}` 形式）。
  - サーバー内部エラー: `500 Internal Server Error`（本文は JSON で `{"error":"internal error"}`）。

- **Body サイズ**: リクエスト Body は 64KB まで。それを超えるとエラーになります。

---

## 転送先の削除 `DELETE /clients`

指定した `webhook_url` の転送先を 1 件削除します。

- **リクエスト**
  - **Content-Type**: `application/json`
  - **Body**: `webhook_url`（必須）— 削除対象の転送先 URL。

- **リクエスト例**:

```bash
curl -s -X DELETE http://localhost:8080/clients \
  -H "Content-Type: application/json" \
  -d '{"webhook_url":"https://example.com/webhook"}'
```

- **レスポンス**:
  - 削除成功: `204 No Content`（本文なし）。
  - 指定した URL が存在しない: `404 Not Found`（本文は JSON で `{"error":"client not found"}`）。
  - `webhook_url` が空または Body が不正: `400 Bad Request`（本文は JSON で `{"error":"..."}` 形式）。
  - サーバー内部エラー: `500 Internal Server Error`（本文は JSON で `{"error":"internal error"}`）。

- **Body サイズ**: リクエスト Body は 64KB まで。

---

## エラー応答の共通形式

`/clients` の 4xx/5xx では、本文が JSON の場合に `{"error":"メッセージ"}` 形式で返ります。`Content-Type` は `application/json; charset=utf-8` です。
