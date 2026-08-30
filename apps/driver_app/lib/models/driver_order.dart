import 'driver_stop.dart';

/// Tipe order yang didukung driver app (ROADMAP 3.10).
enum OrderType {
  ride('ride', 'Ride'),
  food('food', 'Food'),
  send('send', 'Send');

  const OrderType(this.key, this.label);
  final String key;
  final String label;

  static OrderType fromKey(Object? v) {
    final s = (v ?? '').toString().toLowerCase();
    return OrderType.values.firstWhere((e) => e.key == s, orElse: () => OrderType.ride);
  }
}

/// Status aktif (memakan kapasitas driver, max 3).
final Set<String> kActiveRideStatuses = {
  'DRIVER_ASSIGNED',
  'DRIVER_ARRIVED',
  'TRIP_STARTED',
};
final Set<String> kActiveFoodStatuses = {
  'READY_FOR_PICKUP',
  'PICKED_UP',
  'IN_TRANSIT',
};
final Set<String> kActiveSendStatuses = {
  'DRIVER_ASSIGNED',
  'PICKED_UP',
  'IN_TRANSIT',
};

const Set<String> kTerminalOrderStatuses = {'COMPLETED', 'SETTLED', 'CANCELLED', 'DELIVERED'};

/// Model order terpadu untuk dashboard multi-order driver.
///
/// Satu class menampung ride/food/send sehingga dashboard bisa digroup
/// per tipe & hitung kapasitas (>3 tolak accept) dengan satu tipe data.
class DriverOrder {
  const DriverOrder({
    required this.id,
    required this.type,
    required this.status,
    this.driverId,
    this.pickupAddress = '',
    this.pickupLat = 0,
    this.pickupLng = 0,
    this.deliveryAddress = '',
    this.deliveryLat = 0,
    this.deliveryLng = 0,
    this.merchantAddress = '',
    this.merchantName = '',
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.driverEarning = 0,
    this.paymentMethod = 'WALLET',
    this.createdAt,
    this.stops = const [],
    this.isMock = false,
  });

  final String id;
  final OrderType type;
  final String status;
  final String? driverId;
  final String pickupAddress;
  final double pickupLat;
  final double pickupLng;
  final String deliveryAddress;
  final double deliveryLat;
  final double deliveryLng;
  final String merchantAddress;
  final String merchantName;
  final double distanceKm;
  final int estimatedFare;
  final int driverEarning;
  final String paymentMethod;
  final DateTime? createdAt;
  final List<DriverStop> stops;
  final bool isMock;

  bool get hasStops => stops.isNotEmpty;
  int get stopsCompleted => stops.where((s) => s.isDelivered).length;

  bool get isActive => switch (type) {
        OrderType.ride => kActiveRideStatuses.contains(status),
        OrderType.food => kActiveFoodStatuses.contains(status),
        OrderType.send => kActiveSendStatuses.contains(status),
      };

  bool get isTerminal => kTerminalOrderStatuses.contains(status);

  DriverStop? get nextUndeliveredStop {
    for (final s in stops) {
      if (!s.isDelivered) return s;
    }
    return null;
  }

  DriverOrder copyWith({String? status, List<DriverStop>? stops}) {
    return DriverOrder(
      id: id,
      type: type,
      status: status ?? this.status,
      driverId: driverId,
      pickupAddress: pickupAddress,
      pickupLat: pickupLat,
      pickupLng: pickupLng,
      deliveryAddress: deliveryAddress,
      deliveryLat: deliveryLat,
      deliveryLng: deliveryLng,
      merchantAddress: merchantAddress,
      merchantName: merchantName,
      distanceKm: distanceKm,
      estimatedFare: estimatedFare,
      driverEarning: driverEarning,
      paymentMethod: paymentMethod,
      createdAt: createdAt,
      stops: stops ?? this.stops,
      isMock: isMock,
    );
  }

  factory DriverOrder.fromRideJson(Map<String, dynamic> j, {bool isMock = false}) {
    return DriverOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      type: OrderType.ride,
      status: _str(j['status']) ?? 'SEARCHING_DRIVER',
      driverId: _str(j['driver_id']),
      pickupAddress: _str(j['pickup_address']) ?? '',
      pickupLat: _num(j['pickup_lat']),
      pickupLng: _num(j['pickup_lng']),
      deliveryAddress: _str(j['dropoff_address']) ?? '',
      deliveryLat: _num(j['dropoff_lat']),
      deliveryLng: _num(j['dropoff_lng']),
      distanceKm: _num(j['distance_km']),
      estimatedFare: _int(j['estimated_fare']),
      driverEarning: _int(j['driver_earning'] ?? (j['estimated_fare'] is num ? (j['estimated_fare'] as num) * 0.8 : 0)),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      createdAt: _date(j['created_at'] ?? j['expires_at']),
      isMock: isMock,
    );
  }

  factory DriverOrder.fromFoodJson(Map<String, dynamic> j, {bool isMock = false}) {
    final merchant = j['merchant'] is Map ? j['merchant'] as Map : const <String, dynamic>{};
    return DriverOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      type: OrderType.food,
      status: _str(j['status']) ?? 'CONFIRMED',
      driverId: _str(j['driver_id']),
      pickupAddress: _str(j['merchant_address'] ?? merchant['address']) ?? '',
      pickupLat: _num(j['merchant_lat'] ?? merchant['location_lat']),
      pickupLng: _num(j['merchant_lng'] ?? merchant['location_lng']),
      deliveryAddress: _str(j['delivery_address']) ?? '',
      deliveryLat: _num(j['delivery_lat']),
      deliveryLng: _num(j['delivery_lng']),
      merchantName: _str(j['merchant_name'] ?? merchant['name']) ?? '',
      merchantAddress: _str(j['merchant_address'] ?? merchant['address']) ?? '',
      distanceKm: _num(j['distance_km']),
      estimatedFare: _int(j['total_amount'] ?? j['delivery_fee']),
      driverEarning: _int(j['driver_earning'] ?? j['delivery_fee']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      createdAt: _date(j['created_at']),
      isMock: isMock,
    );
  }

  factory DriverOrder.fromSendJson(Map<String, dynamic> j, {bool isMock = false}) {
    final stops = _list(j['stops']).map(DriverStop.fromJson).toList();
    return DriverOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      type: OrderType.send,
      status: _str(j['status']) ?? 'SEARCHING_DRIVER',
      driverId: _str(j['driver_id']),
      pickupAddress: _str(j['pickup_address']) ?? '',
      pickupLat: _num(j['pickup_lat']),
      pickupLng: _num(j['pickup_lng']),
      deliveryAddress: stops.isEmpty ? '' : stops.first.address,
      deliveryLat: stops.isEmpty ? 0 : stops.first.latitude,
      deliveryLng: stops.isEmpty ? 0 : stops.first.longitude,
      distanceKm: _num(j['total_distance_km'] ?? j['distance_km']),
      estimatedFare: _int(j['total_fare'] ?? j['total_amount']),
      driverEarning: _int(j['driver_earning']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      createdAt: _date(j['created_at']),
      stops: stops,
      isMock: isMock,
    );
  }

  factory DriverOrder.fromJson(Map<String, dynamic> j, {bool isMock = false}) {
    final type = OrderType.fromKey(j['type'] ?? j['order_type']);
    return switch (type) {
      OrderType.food => DriverOrder.fromFoodJson(j, isMock: isMock),
      OrderType.send => DriverOrder.fromSendJson(j, isMock: isMock),
      OrderType.ride => DriverOrder.fromRideJson(j, isMock: isMock),
    };
  }

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