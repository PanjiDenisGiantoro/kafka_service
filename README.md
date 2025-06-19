# 🔄 Go-Kafka Data Sync Service

A lightweight and scalable data synchronization service written in **Golang** using **Apache Kafka** for real-time message streaming. Designed for syncing data between **internal microservices** or **external third-party systems** securely and efficiently.

---

## ✨ Features

- 🔗 Real-time data sync between internal/external systems
- 🚀 Built on high-performance Go and Kafka clients
- 🧩 Pluggable architecture for producers and consumers
- 🔒 Secure data handling with TLS and authentication support
- 📦 JSON-based event schema with versioning
- 🛠️ Easy to configure and deploy

---

## 📌 Use Cases

- Synchronizing user profiles between core app and CRM
- Sending event data to external analytics services
- Internal microservices communication (event-driven)
- Logging and audit trails using Kafka as transport

---

## 🧱 Architecture

```text
+----------------+         Kafka         +------------------+
|  Internal App  |  -->  Topic: sync-in  |  External System |
+----------------+                      +------------------+
        |                                        ^
        |                +-------------+         |
        +--------------> |   Go Sync   | --------+
                         |   Service   |
                         +-------------+
                              |
                              v
                      Topic: sync-out (Kafka)
