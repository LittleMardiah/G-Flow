/// Merchant (restaurant) — API_CONTRACT 8.1 (GET /merchants).
class Merchant {
  const Merchant({
    required this.id,
    this.name = '',
    this.category = '',
    this.latitude = 0,
    this.longitude = 0,
    this.address = '',
    this.rating = 0,
    this.isOpen = true,
    this.logoUrl,
    this.deliveryFee = 0,
    this.averageDeliveryTime = 0,
    this.distanceKm = 0,
    this.reviewCount = 0,
  });

  final String id;
  final String name;
  final String category;
  final double latitude;
  final double longitude;
  final String address;
  final double rating;
  final bool isOpen;
  final String? logoUrl;
  final int deliveryFee;
  final int averageDeliveryTime; // menit
  final double distanceKm;
  final int reviewCount;

  factory Merchant.fromJson(Map<String, dynamic> j) {
    return Merchant(
      id: _str(j['merchant_id']) ?? _str(j['id']) ?? '',
      name: _str(j['name']) ?? '',
      category: _str(j['category']) ?? '',
      latitude: _num(j['latitude'] ?? j['lat']),
      longitude: _num(j['longitude'] ?? j['lng'] ?? j['lon']),
      address: _str(j['address']) ?? '',
      rating: _num(j['rating']),
      isOpen: j['is_online'] != false && j['is_open'] != false,
      logoUrl: _str(j['profile_photo'] ?? j['logo_url'] ?? j['image']),
      deliveryFee: _int(j['delivery_fee']),
      averageDeliveryTime: _int(j['average_delivery_time']),
      distanceKm: _num(j['distance_km']),
      reviewCount: _int(j['review_count'] ?? j['total_orders']),
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static double _num(Object? v) => v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}