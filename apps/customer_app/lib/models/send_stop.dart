/// Stop (titik pengantaran) di dalam sebuah send order. Memakai status
/// valid DB 'PENDING/ARRIVED/COMPLETED/SKIPPED' (discrepancy LOGBOOK 3.6).
class SendStop {
  const SendStop({
    required this.id,
    this.orderId,
    this.order = 0,
    this.recipientName = '',
    this.address = '',
    this.latitude = 0,
    this.longitude = 0,
    this.distanceKm = 0,
    this.allocatedFare = 0,
    this.status = 'PENDING',
  });

  final String id;
  final String? orderId;
  final int order;
  final String recipientName;
  final String address;
  final double latitude;
  final double longitude;
  final double distanceKm;
  final int allocatedFare;
  final String status;

  bool get isDelivered => status.toUpperCase() == 'COMPLETED';
  bool get isSkipped => status.toUpperCase() == 'SKIPPED';

  factory SendStop.fromJson(Map<String, dynamic> j) {
    return SendStop(
      id: _str(j['stop_id']) ?? _str(j['id']) ?? '',
      orderId: _str(j['order_id']),
      order: _int(j['order'] ?? j['stop_order']),
      recipientName: _str(j['recipient_name'] ?? j['name']) ?? '',
      address: _str(j['address']) ?? '',
      latitude: _num(j['location_lat'] ?? j['latitude'] ?? j['lat']),
      longitude: _num(j['location_lng'] ?? j['longitude'] ?? j['lng']),
      distanceKm: _num(j['distance_km']),
      allocatedFare: _int(j['allocated_fare']),
      status: _str(j['status']) ?? 'PENDING',
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static double _num(Object? v) => v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}