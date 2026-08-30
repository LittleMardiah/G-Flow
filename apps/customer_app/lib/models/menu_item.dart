import 'item_option.dart';

/// Item menu milik sebuah merchant (API_CONTRACT 8.2).
class MenuItem {
  const MenuItem({
    required this.id,
    this.merchantId,
    this.menuId,
    this.name = '',
    this.description = '',
    this.price = 0,
    this.isAvailable = true,
    this.imageUrl,
    this.options = const [],
  });

  final String id;
  final String? merchantId;
  final String? menuId;
  final String name;
  final String description;
  final int price;
  final bool isAvailable;
  final String? imageUrl;
  final List<ItemOptionGroup> options;

  factory MenuItem.fromJson(Map<String, dynamic> j, {String? merchantId}) {
    return MenuItem(
      id: _str(j['item_id']) ?? _str(j['id']) ?? '',
      merchantId: merchantId ?? _str(j['merchant_id']),
      menuId: _str(j['menu_id']),
      name: _str(j['name']) ?? '',
      description: _str(j['description']) ?? '',
      price: _int(j['price']),
      isAvailable: j['available'] != false && j['is_available'] != false,
      imageUrl: _str(j['image'] ?? j['image_url']),
      options: _list(j['option_groups'])
          .map((g) => ItemOptionGroup.fromJson(g))
          .toList(),
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