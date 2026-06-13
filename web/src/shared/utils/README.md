# Shared Utilities

This directory is reserved for domain-agnostic frontend helpers that can be used by any console layer.

Allowed utilities:

- formatting helpers with no feature knowledge
- small data-shaping helpers for primitive values
- browser-safe helpers that do not call KiwiGuard APIs directly

Do not place API clients, query keys, i18n dictionaries, feature workflows, or application navigation logic here. Those responsibilities belong in `platform`, `features`, `console`, or `app`.
