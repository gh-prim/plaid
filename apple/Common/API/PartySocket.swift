import Foundation
import Observation

/// A refused socket message: the server's code (api/events.md), or
/// `timeout` / `offline` when no answer came.
struct SocketError: Error, Equatable {
    let code: String

    static let timeout = SocketError(code: "timeout")
    static let offline = SocketError(code: "offline")
}

/// The realtime socket (F010, api/events.md): one WebSocket to /api/v1/ws
/// with the session's bearer token. It reconnects with backoff (1, 2, 4… up
/// to 30 s), matches the server's ack and error to the message id, and keeps
/// the server's clock: 5 pings on connect, then one every 30 s, the shortest
/// round trip of the last 8 giving the offset.
@Observable
final class PartySocket {
    enum State: Equatable { case idle, connecting, open, reconnecting }

    private(set) var state: State = .idle
    /// Every server message but replies and pongs: its type and raw JSON.
    var onMessage: (String, Data) -> Void = { _, _ in }
    /// Each time the socket (re)opens, once the first clock ping went out.
    var onOpen: () -> Void = {}

    private var url: URL?
    private var token: String?
    private var session: URLSession?
    private var task: URLSessionWebSocketTask?
    private var listener: Delegate?
    private var attempt = 0
    private var stopped = true
    private var nextId = 0
    private var pending: [String: CheckedContinuation<Void, Error>] = [:]
    private var clockTask: Task<Void, Never>?
    private var reconnectTask: Task<Void, Never>?
    /// (round trip, offset) of the last pongs.
    private var samples: [(rtt: Int64, offset: Int64)] = []
    private var pings: [String: Int64] = [:]
    /// Server time ≈ local time + offset, in ms.
    private(set) var offsetMs: Int64 = 0

    static var localMs: Int64 { Int64(Date().timeIntervalSince1970 * 1000) }

    /// The server's clock now, in Unix ms: the reference for sync.
    var serverNowMs: Int64 { Self.localMs + offsetMs }

    // MARK: Connection

    func start(url: URL, token: String?) {
        guard stopped || self.url != url || self.token != token else { return }
        stop()
        self.url = url
        self.token = token
        stopped = false
        attempt = 0
        connect()
    }

    func stop() {
        stopped = true
        reconnectTask?.cancel()
        clockTask?.cancel()
        task?.cancel(with: .goingAway, reason: nil)
        task = nil
        session?.invalidateAndCancel()
        session = nil
        failPending(.offline)
        state = .idle
    }

    private func connect() {
        guard !stopped, let url else { return }
        state = attempt == 0 ? .connecting : .reconnecting
        var r = URLRequest(url: url)
        if let token { r.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        let delegate = Delegate(owner: self)
        listener = delegate
        session?.invalidateAndCancel()
        let s = URLSession(configuration: .default, delegate: delegate, delegateQueue: nil)
        session = s
        let t = s.webSocketTask(with: r)
        t.maximumMessageSize = 1 << 20
        task = t
        t.resume()
        receive(on: t)
    }

    fileprivate func opened(_ t: URLSessionWebSocketTask) {
        guard t === task else { return }
        attempt = 0
        state = .open
        samples = []
        clockTask?.cancel()
        clockTask = Task { [weak self] in
            for i in 0..<5 {
                self?.ping()
                // Joining needs no clock: rooms rejoin after the first ping.
                if i == 0 { self?.onOpen() }
                try? await Task.sleep(for: .milliseconds(250))
            }
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(30))
                if Task.isCancelled { return }
                self?.ping()
            }
        }
    }

    fileprivate func closed(_ t: URLSessionWebSocketTask) {
        guard t === task else { return }
        task = nil
        clockTask?.cancel()
        failPending(.offline)
        guard !stopped else { return }
        state = .reconnecting
        let delay = min(30, 1 << min(attempt, 5))
        attempt += 1
        reconnectTask?.cancel()
        reconnectTask = Task { [weak self] in
            try? await Task.sleep(for: .seconds(delay))
            guard !Task.isCancelled else { return }
            self?.connect()
        }
    }

    private func receive(on t: URLSessionWebSocketTask) {
        Task { [weak self] in
            while true {
                let message: URLSessionWebSocketTask.Message
                do {
                    message = try await t.receive()
                } catch {
                    self?.closed(t)
                    return
                }
                guard let self else { return }
                switch message {
                case let .string(s): self.handle(Data(s.utf8))
                case let .data(d): self.handle(d)
                @unknown default: break
                }
            }
        }
    }

    // MARK: Messages

    private struct Head: Decodable {
        let type: String
        let id: String?
        let ts: Int64?
        let data: ErrorData?
    }

    private struct ErrorData: Decodable {
        let code: String?
        let t0: Int64?
    }

    private func handle(_ raw: Data) {
        guard let head = try? JSONDecoder().decode(Head.self, from: raw) else { return }
        switch head.type {
        case "ack":
            if let id = head.id { pending.removeValue(forKey: id)?.resume() }
        case "error":
            let code = head.data?.code ?? "internal"
            if let id = head.id, let c = pending.removeValue(forKey: id) { c.resume(throwing: SocketError(code: code)) }
        case "sync.pong":
            guard let id = head.id, let sent = pings.removeValue(forKey: id), let ts = head.ts else { return }
            let received = Self.localMs
            samples.append((received - sent, ts - (sent + received) / 2))
            if samples.count > 8 { samples.removeFirst(samples.count - 8) }
            if let best = samples.min(by: { $0.rtt < $1.rtt }) { offsetMs = best.offset }
        default:
            onMessage(head.type, raw)
        }
    }

    private func ping() {
        nextId += 1
        let id = "p\(nextId)"
        let t0 = Self.localMs
        pings[id] = t0
        if pings.count > 16 { pings = pings.filter { $0.value > t0 - 60_000 } }
        write(Outgoing(type: "sync.ping", id: id, data: ["t0": t0]))
    }

    /// Sends a message and waits for its ack; throws the server's error, or
    /// `timeout` after 10 s.
    func send(_ type: String, _ data: some Encodable) async throws {
        guard state == .open else { throw SocketError.offline }
        nextId += 1
        let id = "c\(nextId)"
        try await withCheckedThrowingContinuation { (c: CheckedContinuation<Void, Error>) in
            pending[id] = c
            write(Outgoing(type: type, id: id, data: data))
            Task { [weak self] in
                try? await Task.sleep(for: .seconds(10))
                self?.pending.removeValue(forKey: id)?.resume(throwing: SocketError.timeout)
            }
        }
    }

    func send(_ type: String) async throws {
        try await send(type, Empty())
    }

    /// Fire and forget: reports and reactions need no answer.
    func post(_ type: String, _ data: some Encodable) {
        guard state == .open else { return }
        write(Outgoing(type: type, id: nil, data: data))
    }

    private func write(_ message: some Encodable) {
        guard let task, let data = try? JSONEncoder().encode(message), let s = String(data: data, encoding: .utf8) else { return }
        task.send(.string(s)) { _ in }
    }

    private func failPending(_ e: SocketError) {
        let all = pending
        pending = [:]
        all.values.forEach { $0.resume(throwing: e) }
    }

    struct Empty: Encodable {}

    private struct Outgoing<D: Encodable>: Encodable {
        let type: String
        let id: String?
        let data: D
    }

    /// URLSession's callbacks, brought back to the main actor.
    private final class Delegate: NSObject, URLSessionWebSocketDelegate, @unchecked Sendable {
        weak var owner: PartySocket?

        init(owner: PartySocket) { self.owner = owner }

        nonisolated func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didOpenWithProtocol protocol: String?) {
            Task { @MainActor in self.owner?.opened(webSocketTask) }
        }

        nonisolated func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask,
                                    didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?) {
            Task { @MainActor in self.owner?.closed(webSocketTask) }
        }
    }
}

/// Decodes a socket message's `data` as T.
func socketData<T: Decodable>(_ raw: Data, as: T.Type = T.self) -> T? {
    try? JSONDecoder().decode(SocketEnvelope<T>.self, from: raw).data
}

private struct SocketEnvelope<T: Decodable>: Decodable {
    let data: T
}
