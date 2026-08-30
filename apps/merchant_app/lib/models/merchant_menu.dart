class MerchantMenu {
  const MerchantMenu({
    required this.id,
    this.merchantId,
    this.name = '',
    this.description,
    this.sequenceOrder = 0,
    this.isActive = true,
    this.items = const [],
  });

  final String id;
  final String? merchantId;
  final String name;
  final String? description;
  final int sequenceOrder;
  final bool isActive;
  final List<Map<String, dynamic>> items;

  MerchantMenu copyWith({String? name, String? description, bool? isActive}) {
    return MerchantMenu(
      id: id,
      merchantId: merchantId,
      name: name ?? this.name,
      description: description ?? this.description,
      sequenceOrder: sequenceOrder,
      isActive: isActive ?? this.isActive,
      items: items,
    );
  }

  factory MerchantMenu.fromJson(Map<String, dynamic> j) {
    return MerchantMenu(
      id: _str(j['menu_id']) ?? _str(j['id']) ?? '',
      merchantId: _str(j['merchant_id']),
      name: _str(j['name']) ?? '',
      description: _str(j['description']),
      sequenceOrder: _int(j['sequence_order']),
      isActive: j['is_active'] != false,
      items: _list(j['items']),
    );
  }

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}