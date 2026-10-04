# AI contribution policy

This is a Human–AI collaborative project. AI systems (for example Grok, OpenAI
ChatGPT / Codex, Claude, Gemini) may assist with design, implementation,
documentation, analysis, and tests. AI is not a human reviewer and does not
authorize regulatory values, verified evidence, device actions, or public
releases.

Changes involving verified-evidence promotion, a regulatory value, device
application, or a public release require a named human reviewer. This rule is
enforced for the repository's Git review path by `routerctl verify commit`.
Repository permissions and any manually created external release remain
separate trust boundaries.

When AI materially assisted a change, record the system actually used:

```text
AI-Assisted-By: Grok
```

or, for other systems:

```text
AI-Assisted-By: OpenAI ChatGPT
AI-Assisted-By: Claude Code
```

Multiple systems may each appear as their own trailer. Do not claim AI
assistance, automation, or human review when it did not occur.

Trailers are separated by role:

```text
Signed-off-by     → human DCO / responsibility (when used)
Co-authored-by    → Git co-author attribution (when used)
AI-Assisted-By    → AI development-assistance provenance
Reviewed-By       → named human review
```
