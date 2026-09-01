import '../models/driver_earning.dart';
import 'api_client.dart';

/// Fallback DEMO untuk earning summary. Endpoint GET /drivers/earnings
/// belum ada di backend (BLUEPRINT) → pakai dataset mock saat
/// kUseMockEarnings=true.
const bool kUseMockEarnings = false;

class EarningsService {
  const EarningsService(this.apiClient);

  final ApiClient apiClient;

  /// GET /drivers/earnings — ringkasan daily/weekly. Fallback mock bila
  /// endpoint belum tersedia.
  Future<DriverEarning> fetchEarnings(String driverId) async {
    if (kUseMockEarnings) return _mockEarnings(driverId);
    final res = await apiClient.get('/api/v1/drivers/earnings?driver_id=$driverId');
    final data = res.data is Map ? (res.data as Map)['data'] : null;
    return DriverEarning.fromJson(
      data is Map<String, dynamic> ? data : const <String, dynamic>{},
    );
  }

  DriverEarning _mockEarnings(String driverId) {
    final now = DateTime.now();
    return DriverEarning(
      todayTotal: 285000,
      todayOrderCount: 12,
      weekTotal: 1675000,
      rideCount: 6,
      foodCount: 5,
      sendCount: 1,
      daily: [7, 8, 9, 10, 11, 12, 13, 14]
          .map((h) => EarningPoint(label: '$h.00', amount: _mockDailyAmount(h)))
          .toList(),
      weekly: List.generate(7, (i) {
        final d = now.subtract(Duration(days: 6 - i));
        final weekday = const ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'][d.weekday - 1];
        return EarningPoint(
          label: weekday,
          amount: _mockWeeklyAmount(i),
          orderCount: 8 + i * 3,
        );
      }),
      isMock: true,
    );
  }

  static int _mockDailyAmount(int hour) {
    if (hour < 9) return 0;
    if (hour < 11) return 12000;
    if (hour < 13) return 35000;
    if (hour < 15) return 42000;
    if (hour < 17) return 48000;
    if (hour < 19) return 55000;
    return 93000;
  }

  static int _mockWeeklyAmount(int index) {
    const amounts = [180000, 210000, 165000, 260000, 240000, 320000, 285000];
    return amounts[index % amounts.length];
  }
}