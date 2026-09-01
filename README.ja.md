# routerctl

[English README](README.md)

Router OS の開発・配備を補助する、クロスプラットフォームのホスト側 CLI です。マニフェスト検証、計画、成果物の解決と検証、規制根拠を保った文書取込などを扱います。ファームウェアやイメージの生成は CLI コアの責務ではありません。

## 重要な安全境界

機種マニフェストの検証・計画・リリース検証は、実機への書込みや RF 動作を許可しません。AX23V の探索用フィクスチャはフラッシュ不可です。ハードウェア・規制根拠・リリースのレビューゲートを回避しないでください。

まずは `routerctl --help` または [CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。詳細な方針は [POLICY.ja.md](POLICY.ja.md) にあります。

利用相談や提案は GitHub Discussions で受け付けます。カテゴリと安全・プライバシー上の注意は [GitHub Discussions の運用方針](docs/community/github-discussions-policy.ja.md) を参照してください。
