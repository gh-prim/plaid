import Foundation
import Network
import Observation

/// A PLAID server seen on the local network.
struct DiscoveredServer: Identifiable, Hashable {
    let id: String
    let name: String
    let version: String?
    let url: URL
}

/// Browses `_plaid._tcp` over Bonjour and resolves each service to a URL.
@Observable
final class ServerDiscovery {
    private(set) var servers: [DiscoveredServer] = []
    private(set) var failed = false

    private var browser: NWBrowser?
    private var resolving: [NWConnection] = []

    func start() {
        guard browser == nil else { return }
        let b = NWBrowser(for: .bonjourWithTXTRecord(type: "_plaid._tcp", domain: nil), using: .tcp)
        b.stateUpdateHandler = { [weak self] state in
            if case .failed = state { Task { @MainActor in self?.failed = true } }
        }
        b.browseResultsChangedHandler = { [weak self] results, _ in
            Task { @MainActor in self?.update(results) }
        }
        b.start(queue: .main)
        browser = b
    }

    func stop() {
        browser?.cancel()
        browser = nil
        resolving.forEach { $0.cancel() }
        resolving = []
    }

    private func update(_ results: Set<NWBrowser.Result>) {
        let names = Set(results.compactMap { r -> String? in
            if case let .service(name, _, _, _) = r.endpoint { return name }
            return nil
        })
        servers.removeAll { !names.contains($0.name) }
        for r in results {
            guard case let .service(name, _, _, _) = r.endpoint,
                  !servers.contains(where: { $0.name == name }) else { continue }
            var txt: [String: String] = [:]
            if case let .bonjour(record) = r.metadata { txt = record.dictionary }
            resolve(r.endpoint, name: name, txt: txt)
        }
    }

    /// Bonjour gives a service name; a short connection gives its address.
    private func resolve(_ endpoint: NWEndpoint, name: String, txt: [String: String]) {
        let params = NWParameters.tcp
        if let ip = params.defaultProtocolStack.internetProtocol as? NWProtocolIP.Options {
            ip.version = .v4
        }
        let c = NWConnection(to: endpoint, using: params)
        c.stateUpdateHandler = { [weak self, weak c] state in
            guard let c else { return }
            switch state {
            case .ready:
                if case let .hostPort(host, port) = c.currentPath?.remoteEndpoint {
                    let h = Self.hostString(host)
                    if let url = URL(string: "http://\(h):\(port.rawValue)") {
                        Task { @MainActor in
                            self?.add(DiscoveredServer(id: txt["id"] ?? name, name: txt["name"] ?? name, version: txt["version"], url: url))
                        }
                    }
                }
                c.cancel()
            case .failed, .cancelled:
                Task { @MainActor in self?.resolving.removeAll { $0 === c } }
            default:
                break
            }
        }
        resolving.append(c)
        c.start(queue: .main)
    }

    private func add(_ server: DiscoveredServer) {
        guard !servers.contains(where: { $0.id == server.id }) else { return }
        servers.append(server)
        servers.sort { $0.name.localizedCompare($1.name) == .orderedAscending }
    }

    nonisolated private static func hostString(_ host: NWEndpoint.Host) -> String {
        switch host {
        case let .ipv4(a): return "\(a)".components(separatedBy: "%").first ?? "\(a)"
        case let .ipv6(a): return "[\("\(a)".components(separatedBy: "%").first ?? "\(a)")]"
        case let .name(n, _): return n
        @unknown default: return "\(host)"
        }
    }
}
