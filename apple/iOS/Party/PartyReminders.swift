import Foundation
import UserNotifications

/// Local reminders 15 minutes before the parties I organize or answered yes
/// or maybe (F010-R29, R41): iOS shows them even with the app closed, until
/// remote push exists (ADR 0010).
enum PartyReminders {
    private static let prefix = "party.reminder."
    static let lead: TimeInterval = 15 * 60

    /// Asks once for the right to notify; quietly does nothing when refused.
    static func ask() async {
        _ = try? await UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge])
    }

    /// One reminder for a party, replacing any earlier one.
    static func schedule(_ party: Party) async {
        let center = UNUserNotificationCenter.current()
        center.removePendingNotificationRequests(withIdentifiers: [prefix + party.id])
        guard let at = party.scheduledAt?.addingTimeInterval(-lead), at > .now else { return }
        let settings = await center.notificationSettings()
        if settings.authorizationStatus == .notDetermined { await ask() }
        let content = UNMutableNotificationContent()
        content.title = tr("party.reminder.title")
        content.body = tr("party.reminder.body %@", party.item?.label ?? tr("vote.tbd"))
        content.sound = .default
        content.userInfo = ["partyId": party.id]
        let when = Calendar.current.dateComponents([.year, .month, .day, .hour, .minute, .second], from: at)
        let request = UNNotificationRequest(identifier: prefix + party.id, content: content,
                                            trigger: UNCalendarNotificationTrigger(dateMatching: when, repeats: false))
        try? await center.add(request)
    }

    static func cancel(_ partyId: String) {
        UNUserNotificationCenter.current().removePendingNotificationRequests(withIdentifiers: [prefix + partyId])
    }

    /// Matches the reminders to the parties the server lists: those I
    /// organize or said yes or maybe to get one, the others lose theirs.
    static func sync(_ parties: [Party], me: String?) async {
        let center = UNUserNotificationCenter.current()
        guard await center.notificationSettings().authorizationStatus == .authorized else { return }
        let wanted = parties.filter { p in
            guard p.status == .scheduled || p.status == .voting, p.scheduledAt != nil else { return false }
            return p.organizer.userId == me || p.myAnswer == "yes" || p.myAnswer == "maybe"
        }
        let keep = Set(wanted.map { prefix + $0.id })
        let pending = await center.pendingNotificationRequests().map(\.identifier).filter { $0.hasPrefix(prefix) }
        center.removePendingNotificationRequests(withIdentifiers: pending.filter { !keep.contains($0) })
        for p in wanted { await schedule(p) }
    }
}

/// A party link (F010-R2): https://<server>/p/<code>, or plaid://party?code=…
/// from the web join page. Nil when url isn't one.
func partyCode(in url: URL) -> String? {
    let c = URLComponents(url: url, resolvingAgainstBaseURL: false)
    if url.scheme == "plaid", url.host() == "party" {
        return c?.queryItems?.first { $0.name == "code" }?.value.map(formattedPartyCode)
    }
    let parts = url.path().split(separator: "/")
    guard url.scheme == "http" || url.scheme == "https", parts.count >= 2, parts[parts.count - 2] == "p" else { return nil }
    let code = String(parts[parts.count - 1]).filter { $0.isLetter || $0.isNumber }
    return code.count == 6 ? formattedPartyCode(code) : nil
}
