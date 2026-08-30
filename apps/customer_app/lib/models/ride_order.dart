import 'package:latlong2/latlong.dart';

/// Urutan status ride yang ditampilkan pada stepper tracking
/// (ROADMAP 02 bagian 3.2 — Customer App).
const List<String> kRideStatusFlow = [
  'SEARCHING_DRIVER',
  'DRIVER_ASSIGNED',
  'DRIVER_ARRIVED',
  'TRIP_STARTED',
  'COMPLETED',
];

/// Status terminal: polling dihentikan & aksi cancel disembunyikan.
const Set<String> kTerminalRideStatuses = {'COMPLETED', 'SETTLED', 'CANCELLED'};

/// Model ride order.
///
/// Disimpan lokal di customer_app (packages/core belum ada di monorepo ini).
/// Field mengikuti response GET /rides/{order_id} pada API_CONTRACT 7.4.
class RideOrder {
  const RideOrder({
    required this.id,
    this.driverId,
    required this.status,
    required this.pickupLat,
    required this.pickupLng,
    required this.dropoffLat,
    required this.dropoffLng,
    this.pickupAddress = '',
    this.dropoffAddress = '',
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.paymentMethod = 'WALLET',
    this.createdAt,
    this.cancellationReason,
  });

  final String id;
  final String? driverId;
  final String status;
  final double pickupLat;
  final double pickupLng;
  final double dropoffLat;
  final double dropoffLng;
  final String pickupAddress;
  final String dropoffAddress;
  final double distanceKm;
  final int estimatedFare;
  final String paymentMethod;
  final DateTime? createdAt;
  final String? cancellationReason;

  factory RideOrder.fromJson(Map<String, dynamic> j) {
    return RideOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      driverId: _str(j['driver_id']),
      status: _str(j['status']) ?? 'SEARCHING_DRIVER',
      pickupLat: _num(j['pickup_lat']),
      pickupLng: _num(j['pickup_lng']),
      dropoffLat: _num(j['dropoff_lat']),
      dropoffLng: _num(j['dropoff_lng']),
      pickupAddress: _str(j['pickup_address']) ?? '',
      dropoffAddress: _str(j['dropoff_address']) ?? '',
      distanceKm: _num(j['distance_km']),
      estimatedFare: _int(j['estimated_fare']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      createdAt: _date(j['created_at']),
      cancellationReason: _str(j['cancellation_reason']),
    );
  }

  RideOrder copyWith({String? status, String? cancellationReason}) {
    return RideOrder(
      id: id,
      driverId: driverId,
      status: status ?? this.status,
      pickupLat: pickupLat,
      pickupLng: pickupLng,
      dropoffLat: dropoffLat,
      dropoffLng: dropoffLng,
      pickupAddress: pickupAddress,
      dropoffAddress: dropoffAddress,
      distanceKm: distanceKm,
      estimatedFare: estimatedFare,
      paymentMethod: paymentMethod,
      createdAt: createdAt,
      cancellationReason: cancellationReason ?? this.cancellationReason,
    );
  }

  LatLng get pickup => LatLng(pickupLat, pickupLng);
  LatLng get dropoff => LatLng(dropoffLat, dropoffLng);

  bool get isTerminal => kTerminalRideStatuses.contains(status);
  int get currentStepIndex => kRideStatusFlow.indexOf(status);

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static double _num(Object? v) => v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

/// Info driver yang ditampilkan di kartu driver pada tracking screen.
class DriverInfo {
  const DriverInfo({
    required this.id,
    this.name = '',
    this.phone = '',
    this.vehicleType = '',
    this.vehiclePlate = '',
    this.photoUrl,
    this.rating = 0,
    this.reviewCount = 0,
  });

  final String id;
  final String name;
  final String phone;
  final String vehicleType;
  final String vehiclePlate;
  final String? photoUrl;
  final double rating;
  final int reviewCount;

  factory DriverInfo.fromJson(Map<String, dynamic> j) {
    return DriverInfo(
      id: (j['driver_id'] ?? j['id'] ?? '').toString(),
      name: (j['name'] ?? j['full_name'] ?? '').toString(),
      phone: (j['phone'] ?? '').toString(),
      vehicleType: (j['vehicle_type'] ?? '').toString(),
      vehiclePlate: (j['vehicle_plate'] ?? '').toString(),
      photoUrl: j['photo_url']?.toString(),
      rating: (j['rating'] is num ? j['rating'] as num : 0).toDouble(),
      reviewCount: (j['review_count'] is num ? j['review_count'] as num : 0).round(),
    );
  }
}

/// Hasil POST /rides/book (API_CONTRACT 7.1). isMock=true menandakan hasil
/// berasal dari simulasi (backend endpoint tracking belum tersedia).
class BookRideResult {
  const BookRideResult({
    required this.orderId,
    required this.status,
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.paymentMethod = 'WALLET',
    this.isMock = false,
  });

  final String orderId;
  final String status;
  final double distanceKm;
  final int estimatedFare;
  final String paymentMethod;
  final bool isMock;

  factory BookRideResult.fromJson(Map<String, dynamic> j, {bool isMock = false}) {
    return BookRideResult(
      orderId: (j['order_id'] ?? '').toString(),
      status: (j['status'] ?? 'SEARCHING_DRIVER').toString(),
      distanceKm: (j['distance_km'] is num ? j['distance_km'] as num : 0).toDouble(),
      estimatedFare: (j['estimated_fare'] is num ? j['estimated_fare'] as num : 0).round(),
      paymentMethod: (j['payment_method'] ?? 'WALLET').toString(),
      isMock: isMock,
    );
  }
}

/// Argumen navigasi ke `/ride-tracking`. Diteruskan via
/// `Navigator.pushNamed(context, AppRoutes.rideTracking, arguments: ...)`
/// agar tracking screen tetap berfungsi walaupun GET /rides/{id} backend
/// belum mengembalikan koordinat (fallback mock menggunakan nilai ini).
class RideTrackingArgs {
  const RideTrackingArgs({
    required this.orderId,
    required this.pickupLat,
    required this.pickupLng,
    required this.dropoffLat,
    required this.dropoffLng,
    this.pickupAddress = '',
    this.dropoffAddress = '',
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.paymentMethod = 'WALLET',
    this.isMock = false,
  });

  final String orderId;
  final double pickupLat;
  final double pickupLng;
  final double dropoffLat;
  final double dropoffLng;
  final String pickupAddress;
  final String dropoffAddress;
  final double distanceKm;
  final int estimatedFare;
  final String paymentMethod;
  final bool isMock;
}