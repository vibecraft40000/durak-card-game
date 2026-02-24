# Error UX Matrix

| Error | errorCode | UX |
|-------|-----------|-----|
| version_mismatch | VERSION_MISMATCH | silent resync, input lock |
| not your turn / invalid turn | INVALID_TURN | micro shake карты |
| invalid card / card not in hand | INVALID_CARD | shake карты, return to hand |
| card does not beat | INVALID_CARD | shake карты |
| connection lost | CONNECTION_LOST | overlay «Переподключение…» |
| timeout applied | TIMEOUT_APPLIED | subtle indicator |
| rate limit exceeded | RATE_LIMIT | краткое сообщение |
