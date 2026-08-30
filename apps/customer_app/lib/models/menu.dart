import 'menu_item.dart';

/// Menu (seksi/kategori) dari sebuah merchant berisi daftar item
/// (API_CONTRACT 8.2, GET /merchants/{merchant_id}/menu).
class Menu {
  const Menu({
    required this.id,
    this.merchantId,
    this.name = '',
    this.items = const [],
  });

  final String id;
  final String? merchantId;
  final String name;
  final List<MenuItem> items;

  factory Menu.fromJson(Map<String, dynamic> j, {String? merchantId}) {
    return Menu(
      id: _str(j['menu_id']) ?? _str(j['id']) ?? '',
      merchantId: merchantId ?? _str(j['merchant_id']),
      name: _str(j['name']) ?? '',
      items: _list(j['items'])
          .map((i) => MenuItem.fromJson(i, merchantId: merchantId))
          .toList(),
    );
  }

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
}