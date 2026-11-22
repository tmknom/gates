package revocation

// TokenStateName は、メインの処理がトークンをジョブの終了時の失効へ引き継ぐ GITHUB_STATE の名前である。
// post-entrypoint は Runner から STATE_<名前> の環境変数で受け取る。
// 書く側と読む側で名前が食い違うと、ジョブの終了時の失効が何もしないまま成功で終えるため、1 か所で定める。
const TokenStateName = "token"
