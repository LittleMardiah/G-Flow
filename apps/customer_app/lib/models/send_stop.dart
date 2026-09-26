/// Stop (titik pengantaran) di dalam sebuah send order. Memakai status
/// valid DB 'PENDING/ARRIVED/COMPLETED/SKIPPED' (discrepancy LOGBOOK 3.6).
///
/// Wire backend mengikuti internal/send.SendOrderStop (TD-126):
/// `stop_number`, `dropoff_lat`, `dropoff_lng`, `dropoff_address`. Nilai
/// decimal di-marshal backend sebagai JSON string (shopspring/decimal, default
/// `MarshalJSONWithoutQuotes=false`), jadi parser harus toleran string.
class SendStop {
  const SendStop({
    required this.id,
    this.orderId,
    this.order = 0,
    this.recipientName = '',
    this.dropoffAddress,
    this.dropoffLat,
    this.dropoffLng,
    this.distanceKm = 0,
    this.allocatedFare = 0,
    this.status = 'PENDING',
  });

  final String id;
  final String? orderId;
  final int order;
  final String recipientName;
  final String? dropoffAddress;
  final double? dropoffLat;
  final double? dropoffLng;
  final double distanceKm;
  final int allocatedFare;
  final String status;

  /// Alias lama (sebelum TD-126) — key tersebut tidak ada di wire backend,
  /// jadi alias ke field dropoff_* supaya call site lama tidak rusak.
  String get address => dropoffAddress ?? '';
  double get latitude => dropoffLat ?? 0;
  double get longitude => dropoffLng ?? 0;

  bool get isDelivered => status.toUpperCase() == 'COMPLETED';
  bool get isSkipped => status.toUpperCase() == 'SKIPPED';

  /// Label lokasi stop untuk UI: alamat bila ada, selain itu koordinat.
  String? get locationLabel {
    final address = dropoffAddress;
    if (address != null) return address;
    final lat = dropoffLat;
    final lng = dropoffLng;
    if (lat == null || lng == null || (lat == 0 && lng == 0)) return null;
    return '${lat.toStringAsFixed(5)}, ${lng.toStringAsFixed(5)}';
  }

  factory SendStop.fromJson(Map<String, dynamic> j) {
    return SendStop(
      id: _str(j['stop_id']) ?? _str(j['id']) ?? '',
      orderId: _str(j['order_id']),
      order: _int(j['stop_number'] ?? j['order'] ?? j['stop_order']),
      recipientName: _str(j['recipient_name'] ?? j['name']) ?? '',
      dropoffAddress: _optStr(
          j['dropoff_address'] ?? j['delivery_address'] ?? j['address']),
      dropoffLat: _optNum(j['dropoff_lat'] ??
          j['delivery_lat'] ??
          j['location_lat'] ??
          j['latitude'] ??
          j['lat']),
      dropoffLng: _optNum(j['dropoff_lng'] ??
          j['delivery_lng'] ??
          j['location_lng'] ??
          j['longitude'] ??
          j['lng']),
      distanceKm: _num(j['distance_km']),
      allocatedFare: _int(j['allocated_fare']),
      status: _str(j['status']) ?? 'PENDING',
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static String? _optStr(Object? v) {
    if (v == null) return null;
    final s = (v is String ? v : v.toString()).trim();
    return s.isEmpty ? null : s;
  }

  static double _num(Object? v) => v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static double? _optNum(Object? v) {
    if (v == null) return null;
    return v is num ? v.toDouble() : double.tryParse('$v');
  }

  /// Decimal backend bisa berupa string bertitik ("7500.00") → parse tolerant.
  static int _int(Object? v) {
    if (v is num) return v.round();
    return int.tryParse('$v') ?? double.tryParse('$v')?.round() ?? 0;
  }
}
