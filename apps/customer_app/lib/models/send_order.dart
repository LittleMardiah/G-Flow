import 'send_stop.dart';

/// Urutan status send order untuk timeline (API_CONTRACT 9.3 state machine).
const List<String> kSendStatusFlow = [
  'CREATED',
  'SEARCHING_DRIVER',
  'DRIVER_ASSIGNED',
  'PICKED_UP',
  'IN_TRANSIT',
  'DELIVERED',
  'SETTLED',
];

/// Status terminal send order.
const Set<String> kTerminalSendStatuses = {'DELIVERED', 'SETTLED', 'CANCELLED'};

/// Send order (API_CONTRACT 9.1 / 9.2 / 9.3 / 9.4).
class SendOrder {
  const SendOrder({
    required this.id,
    this.customerId,
    this.driverId,
    required this.status,
    this.packageType = 'STANDARD',
    this.weightKg = 0,
    this.totalDistanceKm = 0,
    this.estimatedFare = 0,
    this.totalAmount = 0,
    this.paymentMethod = 'WALLET',
    this.pickupAddress = '',
    this.createdAt,
    this.stops = const [],
    this.isMock = false,
  });

  final String id;
  final String? customerId;
  final String? driverId;
  final String status;
  final String packageType;
  final double weightKg;
  final double totalDistanceKm;
  final int estimatedFare;
  final int totalAmount;
  final String paymentMethod;
  final String pickupAddress;
  final DateTime? createdAt;
  final List<SendStop> stops;
  final bool isMock;

  factory SendOrder.fromJson(Map<String, dynamic> j, {bool isMock = false}) {
    return SendOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      customerId: _str(j['customer_id']) ?? _str(j['sender_id']),
      driverId: _str(j['driver_id']),
      status: _str(j['status']) ?? 'SEARCHING_DRIVER',
      packageType: _str(j['package_type']) ?? 'STANDARD',
      weightKg: _num(j['weight_kg']),
      totalDistanceKm: _num(j['total_distance_km']),
      estimatedFare: _int(j['estimated_fare'] ?? j['total_fare']),
      totalAmount: _int(j['total_amount'] ?? j['total_fare']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      pickupAddress: _str(j['pickup_address']) ?? '',
      createdAt: _date(j['created_at']),
      stops: _list(j['stops']).map(SendStop.fromJson).toList(),
      isMock: isMock,
    );
  }

  int get stopsCompleted => stops.where((s) => s.isDelivered).length;
  bool get isTerminal => kTerminalSendStatuses.contains(status);
  int get currentStepIndex => kSendStatusFlow.indexOf(status);

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static double _num(Object? v) => v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

/// Argumen navigasi ke /send-tracking (fallback saat detail backend belum
/// tersedia lengkap).
class SendOrderTrackingArgs {
  const SendOrderTrackingArgs({
    required this.orderId,
    this.totalAmount = 0,
    this.packageType = 'STANDARD',
    this.paymentMethod = 'WALLET',
    this.isMock = false,
  });

  final String orderId;
  final int totalAmount;
  final String packageType;
  final String paymentMethod;
  final bool isMock;
}