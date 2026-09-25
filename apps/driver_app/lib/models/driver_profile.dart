class DriverProfile {
  const DriverProfile({
    required this.driverId,
    this.name = '',
    this.email = '',
    this.phone,
    this.vehicleType,
    this.vehiclePlate,
    this.licenseNumber,
    this.licenseExpiry,
    this.status = '',
    this.ratingAvg = 0,
    this.totalRides = 0,
    this.bankName,
    this.bankAccountMasked,
  });

  final String driverId;
  final String name;
  final String email;
  final String? phone;
  final String? vehicleType;
  final String? vehiclePlate;
  final String? licenseNumber;
  final String? licenseExpiry;
  final String status;
  final double ratingAvg;
  final int totalRides;
  final String? bankName;
  final String? bankAccountMasked;

  factory DriverProfile.fromJson(Map<String, dynamic> json) {
    return DriverProfile(
      driverId: _string(json['driver_id'] ?? json['id']) ?? '',
      name: _string(json['name']) ?? '',
      email: _string(json['email']) ?? '',
      phone: _string(json['phone']),
      vehicleType: _string(json['vehicle_type']),
      vehiclePlate: _string(json['vehicle_plate']),
      licenseNumber: _string(json['license_number']),
      licenseExpiry: _string(json['license_expiry']),
      status: _string(json['status']) ?? '',
      ratingAvg: _number(json['rating_avg'])?.toDouble() ?? 0,
      totalRides: _number(json['total_rides'])?.toInt() ?? 0,
      bankName: _string(json['bank_name']),
      bankAccountMasked: _string(json['bank_account_masked']),
    );
  }

  static DriverProfile fromLocal({
    required String driverId,
    String? name,
    String? email,
    String? phone,
    String? vehicleType,
    String? vehiclePlate,
  }) {
    return DriverProfile(
      driverId: driverId,
      name: name ?? '',
      email: email ?? '',
      phone: phone,
      vehicleType: vehicleType,
      vehiclePlate: vehiclePlate,
    );
  }

  bool get hasVehicleInfo =>
      (vehicleType != null && vehicleType!.isNotEmpty) ||
      (vehiclePlate != null && vehiclePlate!.isNotEmpty);

  static String? _string(Object? value) {
    if (value == null) return null;
    final text = value.toString().trim();
    return text.isEmpty ? null : text;
  }

  static num? _number(Object? value) {
    if (value is num) return value;
    if (value is String) return num.tryParse(value);
    return null;
  }
}
