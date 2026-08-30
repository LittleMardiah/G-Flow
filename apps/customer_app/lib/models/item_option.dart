import 'option_choice.dart';

/// Grup opsi item (variants) — HALAMAN v2.0-FIXED #3 "Multi-Group Variant
/// Options". Suatu item dapat memiliki banyak grup (mis. "Level Pedas" dan
/// "Ukuran"). selection_type: SINGLE (radio) / MULTIPLE (checkbox).
class ItemOptionGroup {
  const ItemOptionGroup({
    required this.id,
    this.name = '',
    this.selectionType = 'SINGLE',
    this.isRequired = false,
    this.options = const [],
  });

  final String id;
  final String name;
  final String selectionType; // SINGLE | MULTIPLE
  final bool isRequired;
  final List<OptionChoice> options;

  bool get isMultiple => selectionType.toUpperCase() == 'MULTIPLE';

  factory ItemOptionGroup.fromJson(Map<String, dynamic> j) {
    return ItemOptionGroup(
      id: _str(j['option_group_id']) ?? _str(j['id']) ?? '',
      name: _str(j['name']) ?? '',
      selectionType:
          _str(j['selection_type']) ?? _str(j['selectionType']) ?? 'SINGLE',
      isRequired: j['required'] == true || j['is_required'] == true,
      options: _list(j['options']).map(OptionChoice.fromJson).toList(),
    );
  }

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
}
