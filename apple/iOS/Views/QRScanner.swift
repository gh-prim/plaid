import SwiftUI
import VisionKit

/// The camera, looking for the QR code a TV shows while it waits to be
/// paired; hands back the pairing code it holds.
struct QRScannerSheet: View {
    let found: (String) -> Void
    @Environment(\.dismiss) private var dismiss

    /// The device has a camera that can read codes, and the app may use it.
    static var available: Bool { DataScannerViewController.isSupported && DataScannerViewController.isAvailable }

    var body: some View {
        NavigationStack {
            Scanner { payload in
                guard let code = pairingCode(in: payload) else { return false }
                found(code)
                dismiss()
                return true
            }
            .ignoresSafeArea()
            .overlay(alignment: .bottom) {
                Text("pairing.scan.hint")
                    .font(Theme.sans(14, .semibold))
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 18).padding(.vertical, 12)
                    .glassEffect(.regular, in: Capsule())
                    .padding(.bottom, 40)
                    .padding(.horizontal, 20)
            }
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button(role: .close) { dismiss() } }
            }
        }
    }

    private struct Scanner: UIViewControllerRepresentable {
        /// Returns true once a payload was a pairing code.
        let onPayload: (String) -> Bool

        func makeUIViewController(context: Context) -> DataScannerViewController {
            let vc = DataScannerViewController(recognizedDataTypes: [.barcode(symbologies: [.qr])],
                                               qualityLevel: .balanced, isHighlightingEnabled: true)
            vc.delegate = context.coordinator
            try? vc.startScanning()
            return vc
        }

        func updateUIViewController(_ vc: DataScannerViewController, context: Context) {}

        static func dismantleUIViewController(_ vc: DataScannerViewController, coordinator: Coordinator) {
            vc.stopScanning()
        }

        func makeCoordinator() -> Coordinator { Coordinator(onPayload: onPayload) }

        final class Coordinator: NSObject, DataScannerViewControllerDelegate {
            let onPayload: (String) -> Bool
            private var done = false

            init(onPayload: @escaping (String) -> Bool) { self.onPayload = onPayload }

            func dataScanner(_ scanner: DataScannerViewController, didAdd items: [RecognizedItem], allItems: [RecognizedItem]) {
                guard !done else { return }
                for case let .barcode(code) in items {
                    if let payload = code.payloadStringValue, onPayload(payload) {
                        done = true
                        UINotificationFeedbackGenerator().notificationOccurred(.success)
                        return
                    }
                }
            }
        }
    }
}

/// The pairing code in a TV's QR code (…/pair?code=K7P4QX), or the code
/// itself; nil for anything else.
func pairingCode(in payload: String) -> String? {
    let alphabet = Set("ABCDEFGHJKMNPQRSTUVWXYZ23456789")
    func clean(_ s: String) -> String? {
        let c = s.uppercased().filter { $0 != "-" }
        return c.count == 6 && c.allSatisfy(alphabet.contains) ? c : nil
    }
    if let url = URLComponents(string: payload), url.path.hasSuffix("/pair"),
       let code = url.queryItems?.first(where: { $0.name == "code" })?.value {
        return clean(code)
    }
    return clean(payload.trimmingCharacters(in: .whitespaces))
}
