class MerchantProfile {
  const MerchantProfile({
    required this.id,
    this.userId,
    this.merchantName = '',
    this.category = '',
    this.address = '',
    this.latitude = 0,
    this.longitude = 0,
    this.phone,
    this.openingTime,
    this.closingTime,
    this.isOpen = true,
    this.status = 'PENDING_VERIFICATION',
    this.logoUrl,
    this.avgRating = 0,
    this.totalReviews = 0,
    this.totalOrders = 0,
  });

  final String id;
  final String? userId;
  final String merchantName;
  final String category;
  final String address;
  final double latitude;
  final double longitude;
  final String? phone;
  final String? openingTime;
  final String? closingTime;
  final bool isOpen;
  final String status;
  final String? logoUrl;
  final double avgRating;
  final int totalReviews;
  final int totalOrders;

  bool get isActive => status.toUpperCase() == 'ACTIVE';
  bool get isPendingVerification =>
      status.toUpperCase() == 'PENDING_VERIFICATION';

  MerchantProfile copyWith({
    bool? isOpen,
    String? merchantName,
    String? category,
    String? openingTime,
    String? closingTime,
    String? logoUrl,
    String? address,
  }) {
    return MerchantProfile(
      id: id,
      userId: userId,
      merchantName: merchantName ?? this.merchantName,
      category: category ?? this.category,
      address: address ?? this.address,
      latitude: latitude,
      longitude: longitude,
      phone: phone,
      openingTime: openingTime ?? this.openingTime,
      closingTime: closingTime ?? this.closingTime,
      isOpen: isOpen ?? this.isOpen,
      status: status,
      logoUrl: logoUrl ?? this.logoUrl,
      avgRating: avgRating,
      totalReviews: totalReviews,
      totalOrders: totalOrders,
    );
  }

  factory MerchantProfile.fromJson(Map<String, dynamic> j) {
    return MerchantProfile(
      id: _str(j['merchant_id']) ?? _str(j['id']) ?? '',
      userId: _str(j['user_id']),
      merchantName: _str(j['merchant_name'] ?? j['name']) ?? '',
      category: _str(j['category']) ?? '',
      address: _str(j['address']) ?? '',
      latitude: _num(j['latitude'] ?? j['lat']),
      longitude: _num(j['longitude'] ?? j['lng'] ?? j['lon']),
      phone: _str(j['phone']),
      openingTime: _str(j['opening_time']),
      closingTime: _str(j['closing_time']),
      isOpen: j['is_open'] != false,
      status: _str(j['status']) ?? 'PENDING_VERIFICATION',
      logoUrl: _str(j['logo_url'] ?? j['profile_photo']),
      avgRating: _num(j['avg_rating']),
      totalReviews: _int(j['total_reviews']),
      totalOrders: _int(j['total_orders']),
    );
  }

  Map<String, dynamic> toUpdateJson({
    bool? isOpen,
    String? merchantName,
    String? category,
    String? openingTime,
    String? closingTime,
    String? logoUrl,
  }) {
    return {
      'is_open': ?isOpen,
      'merchant_name': ?merchantName,
      'category': ?category,
      'opening_time': ?openingTime,
      'closing_time': ?closingTime,
      'logo_url': ?logoUrl,
    };
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static double _num(Object? v) =>
      v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}