/// Pilihan di dalam sebuah grup opsi item (mis. "Extra Cheese" pada grup
/// "Extra Topping"). Mengikuti API_CONTRACT 8.2 (GET /merchants/{id}/menu).
class OptionChoice {
  const OptionChoice({
    required this.id,
    this.name = '',
    this.priceAdjustment = 0,
  });

  final String id;
  final String name;
  final int priceAdjustment;

  factory OptionChoice.fromJson(Map<String, dynamic> j) {
    return OptionChoice(
      id: _str(j['option_id']) ?? _str(j['id']) ?? '',
      name: _str(j['name']) ?? '',
      priceAdjustment: _int(j['price_adjust'] ?? j['price_adjustment']),
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}
