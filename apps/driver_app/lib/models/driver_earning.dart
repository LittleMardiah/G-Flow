/// Ringkasan penghasilan driver (ROADMAP 3.10: earnings provider/screen).
class EarningPoint {
  const EarningPoint({
    required this.label,
    required this.amount,
    this.orderCount = 0,
  });

  final String label;
  final int amount;
  final int orderCount;

  factory EarningPoint.fromJson(Map<String, dynamic> j) {
    return EarningPoint(
      label: _str(j['label'] ?? j['date'] ?? '') ?? '',
      amount: _int(j['amount'] ?? j['total']),
      orderCount: _int(j['order_count'] ?? j['orders']),
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}

/// Ringkasan harian/mingguan: total earnings, jumlah order per tipe,
/// dan data series untuk bar-chart fl_chart (konsisten antar periode).
class DriverEarning {
  const DriverEarning({
    this.todayTotal = 0,
    this.todayOrderCount = 0,
    this.weekTotal = 0,
    this.rideCount = 0,
    this.foodCount = 0,
    this.sendCount = 0,
    this.daily = const [],
    this.weekly = const [],
    this.isMock = false,
  });

  final int todayTotal;
  final int todayOrderCount;
  final int weekTotal;
  final int rideCount;
  final int foodCount;
  final int sendCount;
  final List<EarningPoint> daily;
  final List<EarningPoint> weekly;
  final bool isMock;

  int get totalOrderCount => rideCount + foodCount + sendCount;

  factory DriverEarning.fromJson(Map<String, dynamic> j, {bool isMock = false}) {
    return DriverEarning(
      todayTotal: _int(j['today_total'] ?? j['total_earnings']),
      todayOrderCount: _int(j['today_order_count'] ?? j['order_count']),
      weekTotal: _int(j['week_total']),
      rideCount: _int(j['ride_count']),
      foodCount: _int(j['food_count']),
      sendCount: _int(j['send_count']),
      daily: _list(j['daily']).map(EarningPoint.fromJson).toList(),
      weekly: _list(j['weekly']).map(EarningPoint.fromJson).toList(),
      isMock: isMock,
    );
  }

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}