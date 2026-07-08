<div align="center">

# Go Wind

### ミニマルでコンポーザブルな Go マイクロサービスフレームワーク

レゴ型アーキテクチャ · インターフェース駆動 · ゼロマジック · 本番環境対応

[中文](./README.md) · [English](./README_en.md) · 日本語

</div>

---

## 設計哲学

> **オールインワンではなく、ブロックの箱。**

go-wind は Go のネイティブな哲学を掲げています：**継承より合成、実装よりインターフェース。** フレームワークはプロトコルとライフサイクルの骨組みのみを定義し、インフラストラクチャへの依存は一切ありません。各モジュール（トランスポート、レジストリ、ログ）は最小限のインターフェースを公開し、利用者がレゴブロックのように組み立てます。

| オールインワンフレームワーク | go-wind |
|:---:|:---:|
| gRPC + etcd + zap にロックイン | gRPC か HTTP かはあなたが決める |
| フレームワークがすべてを管理 | フレームワークはライフサイクルのみ管理 |
| アップグレード = 全スタックの更新 | アップグレード = 骨組みのみの更新 |
| 学習コストが高い | ソースコードを5分で読了 |

---

## 主要機能

- **コンポーザブルな組み立て** — コアはライフサイクルのみ管理、transport/log は最小インターフェースのみ公開。レジストリ、設定センター等は [go-wind-plugins](https://github.com/tx7do/go-wind-plugins) が具体的な実装を提供
- **グレースフルなライフサイクル** — シグナル検知、タイムアウト制御付きサーバー起動/停止；単一サーバーのクラッシュが全サーバーのグレースフルシャットダウンをカスケード
- **非侵入型 Context** — TraceID / UserID / ColorTag は context 経由で伝播、data race を防ぐためディープコピーを実施
- **ミニマルログファサード** — 4メソッドインターフェース + `Enabled` + `With`、数行で slog / zap / zerolog / kratos log を適応可能。具象アダプター（slog adapter、LevelFilter、MultiLogger）は [go-wind-plugins](https://github.com/tx7do/go-wind-plugins) が提供
- **関数型オプション** — `WithServer`、`WithName`… チェーン可能、型安全、可読性が高い
- **外部依存ゼロ** — `golang.org/x/sync` のみ、フレームワーク本体は500行未満

---

## クイックスタート

### インストール

```bash
go get github.com/tx7do/go-wind
```

### 最小サンプル

```go
package main

import (
    "context"
    "log"

    wind "github.com/tx7do/go-wind"
    "github.com/tx7do/go-wind/transport"
)

// MyServer は transport.Server インターフェースを実装
type MyServer struct{}

func (s *MyServer) Start(ctx context.Context) error {
    <-ctx.Done()
    return ctx.Err()
}

func (s *MyServer) Stop(ctx context.Context) error {
    // クリーンアップ処理（ctx はタイムアウトを保持）
    return nil
}

func (s *MyServer) Endpoint() string {
    return "grpc://0.0.0.0:9000"
}

func main() {
    app := wind.New(
        wind.WithID("order-service-01"),
        wind.WithName("order-service"),
        wind.WithVersion("v1.0.0"),
        wind.WithServer(&MyServer{}),
    )

    if err := app.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### 複数サーバーの組み合わせ

```go
app := wind.New(
    wind.WithName("gateway"),
    wind.WithServer(grpcServer, httpServer, wsServer),
)

// 3つのサーバーが並行起動し、シグナル受信後に並行グレースフル停止
app.Run(ctx)
```

### サービスインスタンス構築

```go
app := wind.New(
    wind.WithName("user-service"),
    wind.WithServer(grpcServer),
)

// 選択したレジストリ実装（go-wind-plugins）に登録するためのインスタンスを構築
inst := app.Instance("grpc://0.0.0.0:9000")
// inst.ID / inst.Name / inst.Version / inst.Endpoints

app.Run(ctx)
```

### ログ統合

```go
import windlog "github.com/tx7do/go-wind/log"

// 方式1: log.Logger インターフェースを実装して、独自のバックエンドを適応
// 具象アダプター（slog adapter、LevelFilter、MultiLogger）は go-wind-plugins が提供
windlog.SetLogger(myZapAdapter{})

// 方式2: go-wind-plugins/log/slog のアダプターを使用
//   import pluginslog "github.com/tx7do/go-wind-plugins/log/slog"
//   windlog.SetLogger(pluginslog.New(mySlogLogger))

// App レベルロガー、未設定時はグローバルロガーにフォールバック
app.Logger().Info(ctx, "starting")

// 高コストな引数構築を Enabled でガード
logger := app.Logger()
if logger.Enabled(windlog.LevelDebug) {
    logger.Debug(ctx, "detail", computeExpensiveData())
}
```

### 高度な設定

```go
app := wind.New(
    wind.WithServer(grpcServer),
    wind.WithStopTimeout(30*time.Second),  // グレースフルシャットダウンのタイムアウト
    wind.WithSignal(syscall.SIGTERM),       // カスタムシグナル
    wind.WithLogger(myLogger),              // App レベルのロガー
    wind.WithBeforeStop(func(ctx context.Context) error {
        // シャットダウン前フック：サービス登録解除、リクエストキューのドレイン
        // 選択したレジストリ実装（go-wind-plugins）を使用
        return nil
    }),
    wind.WithAfterStop(func(ctx context.Context) error {
        // シャットダウン後フック：DB接続のクローズ、バッファのフラッシュ
        return db.Close()
    }),
)

// App レベルロガー、未設定時はグローバルロガーにフォールバック
app.Logger().Info(ctx, "starting")

// Server.Endpoint() は実際のリスニングアドレスを返す（ランダムポート :0 に対応）
endpoint := grpcServer.Endpoint()

// App の終了を待機（外部監視に利用可能）
<-app.Done()
// Run の最終エラーを取得（Done クローズ後に呼び出し可能）
if err := app.Err(); err != nil {
    log.Fatal(err)
}
```

---

## モジュールアーキテクチャ

```
graph LR
    APP["wind.App<br/>ライフサイクル編成"]
    APP -->|"管理"| SERVER["transport.Server<br/>インターフェース契約"]
    APP -.->|"グローバル fallback"| LOG["log.Logger<br/>インターフェース（組み込み実装なし）"]
```

> コアは Server ライフサイクルのみ管理。ログはインターフェース + グローバル登録器のみ提供。レジストリ、設定センター等は [go-wind-plugins](https://github.com/tx7do/go-wind-plugins) が提供。

```
go-wind/
├── app.go              コアエンジン：App ライフサイクル管理
├── errors.go           エラー定義の一元管理（パッケージレベルのセンティネルエラー）
├── context.go          リクエストスコープメタデータ伝播（TraceID / UserID / Metadata）
├── instance.go         サービスインスタンスモデル
├── errors/             構造化・トランスポート対応エラーモデル（WindError）
├── transport/          トランスポート抽象化（Server）
└── log/                ログファサード（Logger インターフェース + Level + nop 実装 + グローバル登録器）
```

### モジュール概要

| モジュール | コアインターフェース | 責務 |
|:---|:---|:---|
| `wind` | `App`, `Option` | アプリケーションライフサイクル編成、グレースフルシャットダウン |
| `wind` | `Instance` | サービスインスタンスモデリング |
| `wind` | `Metadata` | リクエストスコープメタデータ（TraceID 等）の伝播 |
| `transport` | `Server` | トランスポート層の抽象化、任意プロトコル対応 |
| `log` | `Logger`, `Level` | ログインターフェース契約 + グローバル登録器。アダプターは plugins が提供 |

---

## ライフサイクル & グレースフルシャットダウン

go-wind のコア機能は **信頼性の高いアプリケーションライフサイクル管理** です：

```
graph TB
    subgraph Phase 1: Start
        S1["srv.Start"] --> EG
        S2["srv.Start"] --> EG
        S3["srv.Start"] --> EG
    end

    SIG["シグナル受信<br/>SIGTERM / SIGINT / SIGQUIT"] --> TC
    CTX["ctx キャンセル"] --> TC
    CRASH["サーバークラッシュ / 終了"] --> TC

    TC["triggerCancel()"] -->|"キャンセル"| EG["errgroup<br/>egCtx"]

    EG --> P2["Phase 2: BeforeStop フック<br/>同期実行、Stop より先"]
    P2 --> P3

    subgraph Phase 3: Server.Stop
        P3["並行 Stop"]
        P3 --> Stop1["srv.Stop<br/>独立タイムアウト Ctx"]
        P3 --> Stop2["srv.Stop<br/>独立タイムアウト Ctx"]
        P3 --> Stop3["srv.Stop<br/>独立タイムアウト Ctx"]
    end

    Stop1 --> P4["Phase 4: AfterStop フック<br/>同期実行、Stop の後"]
    Stop2 --> P4
    Stop3 --> P4
```

**設計のポイント：**

| メカニズム | 説明 |
|:---|:---|
| 独立停止コンテキスト | Stop context は `context.Background()` から派生 — 実行 context から**ではなく** — タイムアウトウィンドウが実効性を持ちます |
| クラッシュカスケード | サーバーのクラッシュまたは自己終了が errgroup 経由で他の全サーバーのグレースフルシャットダウンを自動トリガー |
| ダブルストップ防止 | `App.Stop()` はキャンセルのみをトリガーし、`Server.Stop()` を直接呼び出しません — シャットダウンロジックは一元化 |
| シグナル検知 | デフォルトで `SIGTERM` / `SIGINT` / `SIGQUIT` を監視、完全にカスタマイズ可能 |
| ライフサイクルフック | `WithBeforeStop` / `WithAfterStop` が段階的順序で実行：BeforeStop → Server.Stop → AfterStop、各フェーズに独立タイムアウトコンテキスト |
| Server.Endpoint | Server 実装が実際のリスニングアドレスを公開、`:0` ランダムポートバインディング後のレジストリ登録をサポート |
| App.Err | `App.Err()` は `Done()` クローズ後に Run の最終エラーを返し、外部監視が結果を把握可能 |

---

## 設計原則

### 1. ミニマルインターフェース

各インターフェースは必要最小限のメソッドのみを定義します。例えば `Logger` は4つのログメソッド + `Enabled` + `With` のみで、任意のバックエンドへの適応は数行のグルーコードで済みます。`Enabled` メソッドにより、高コストな引数構築前にレベルをチェックできます。

### 2. 暗黙的依存ゼロ

フレームワークはレジストリ、設定センター、ログライブラリについて一切の仮定をしません。`go.mod` の依存は `golang.org/x/sync` のみです。

### 3. Context ネイティブ

すべてのインターフェースの第一引数は `context.Context` で、Go 標準ライブラリの哲学と一致し、トレーシングとタイムアウト伝播をサポートします。

### 4. 並行安全性

グローバル状態（ロガー）とメタデータ伝播は並行安全です。`WithTraceID` は共有マップをディープコピーし、data race を防止します。

---

## 動作環境

| 項目 | 要件 |
|:---|:---|
| Go バージョン | 1.23+ |
| 依存関係 | `golang.org/x/sync` のみ |

---

## ライセンス

[MIT License](./LICENSE)
