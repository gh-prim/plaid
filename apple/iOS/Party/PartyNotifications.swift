import UIKit
import UserNotifications

/// Party reminders (F010-R41): shown even with the app open, and a tap opens
/// the party's page.
extension AppDelegate: UNUserNotificationCenterDelegate {
    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        return true
    }

    nonisolated func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification) async -> UNNotificationPresentationOptions {
        [.banner, .sound]
    }

    nonisolated func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse) async {
        guard let id = response.notification.request.content.userInfo["partyId"] as? String else { return }
        await MainActor.run { NotificationCenter.default.post(name: .openParty, object: id) }
    }
}
