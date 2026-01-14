# CitrineDB

<p align="center">
  <img src="logo.png" alt="CitrineDB Logo" width="400">
</p>

<div align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/go-1.25+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat" alt="License"></a>
  <img src="https://img.shields.io/badge/status-in%20development-orange?style=flat" alt="Status">
</div>

<br>

> ⚠️ **This project is currently under active development and is not ready for production use.**

## About

CitrineDB is an embedded database engine written in Go, inspired by SQLite. It implements a modular storage engine with educational and practical purposes in mind.

## Features (Implemented)

- **Slotted Page Format** - Variable-length record storage in fixed-size pages
- **Buffer Pool** - LRU-based page caching with dirty page tracking
- **Disk Manager** - Low-level file I/O and page allocation
- **B+Tree Index** - Fast key-based lookups using [treego](https://github.com/l00pss/treego)
- **WAL (Write-Ahead Logging)** - Durability and crash recovery using [walrus](https://github.com/l00pss/walrus)
- **Heap File** - Unordered record collection with RID-based access
- **Record Serialization** - Binary encoding for structured data



## Roadmap

- [ ] Catalog layer (schema management)
- [ ] Query parser and planner
- [ ] SQL interface
- [ ] Concurrency control (MVCC)

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

**Vugar Mammadli** - [@l00pss](https://github.com/l00pss)

<div align="center">
  <a href="https://www.buymeacoffee.com/l00pss" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 50px !important;width: 180px !important;" ></a>
</div>