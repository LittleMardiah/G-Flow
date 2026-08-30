import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../models/merchant_item.dart';
import '../providers/menu_provider.dart';

class ItemEditorScreen extends ConsumerStatefulWidget {
  const ItemEditorScreen({super.key, this.itemId, this.initialMenuId});

  final String? itemId;
  final String? initialMenuId;

  @override
  ConsumerState<ItemEditorScreen> createState() => _ItemEditorScreenState();
}

class _ItemEditorScreenState extends ConsumerState<ItemEditorScreen> {
  final _formKey = GlobalKey<FormState>();
  final _name = TextEditingController();
  final _description = TextEditingController();
  final _price = TextEditingController();
  final _stock = TextEditingController();
  final _imageUrl = TextEditingController();
  String? _selectedMenuId;
  bool _isAvailable = true;
  String? _pickedImagePath;
  bool _submitting = false;
  bool _isNew = true;

  @override
  void initState() {
    super.initState();
    final item = _findItem();
    _isNew = widget.itemId == null;
    if (item != null) {
      _name.text = item.name;
      _description.text = item.description ?? '';
      _price.text = '${item.price}';
      _stock.text = '${item.stock}';
      _imageUrl.text = item.imageUrl ?? '';
      _selectedMenuId = item.menuId ?? widget.initialMenuId;
      _isAvailable = item.isAvailable;
    } else {
      _selectedMenuId = widget.initialMenuId ?? _firstMenuId();
    }
  }

  MerchantItem? _findItem() {
    if (widget.itemId == null) return null;
    for (final item in ref.read(menuProvider).items) {
      if (item.id == widget.itemId) return item;
    }
    return null;
  }

  String? _firstMenuId() {
    final menus = ref.read(menuProvider).menus;
    return menus.isNotEmpty ? menus.first.id : null;
  }

  @override
  void dispose() {
    _name.dispose();
    _description.dispose();
    _price.dispose();
    _stock.dispose();
    _imageUrl.dispose();
    super.dispose();
  }

  Future<void> _pickImage() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked == null) return;
    setState(() => _pickedImagePath = picked.path);
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    final menuId = _selectedMenuId ?? _firstMenuId();
    if (menuId == null) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Buat menu terlebih dahulu')));
      return;
    }
    final price = int.tryParse(_price.text.trim()) ?? 0;
    final stock = int.tryParse(_stock.text.trim()) ?? 999;
    final imageUrl = _imageUrl.text.trim().isEmpty ? (_pickedImagePath ?? '') : _imageUrl.text.trim();

    setState(() => _submitting = true);
    final notifier = ref.read(menuProvider.notifier);
    final bool success;
    if (_isNew) {
      success = await notifier.createItem(
        menuId: menuId,
        name: _name.text.trim(),
        description: _description.text.trim(),
        price: price,
        imageUrl: imageUrl,
        stock: stock,
        isAvailable: _isAvailable,
      );
    } else {
      success = await notifier.updateItem(
        widget.itemId!,
        name: _name.text.trim(),
        description: _description.text.trim(),
        price: price,
        imageUrl: imageUrl,
        stock: stock,
        isAvailable: _isAvailable,
      );
    }
    if (!mounted) return;
    setState(() => _submitting = false);
    if (success) {
      Navigator.of(context).pop();
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Gagal menyimpan: ${ref.read(menuProvider).error ?? 'unknown'}')),
      );
    }
  }

  Future<void> _delete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Hapus Item?'),
        content: const Text('Item akan dihapus permanen.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Batal')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Theme.of(context).colorScheme.error),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    final success = await ref.read(menuProvider.notifier).deleteItem(widget.itemId!);
    if (!mounted) return;
    if (success) {
      Navigator.of(context).pop();
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Gagal menghapus: ${ref.read(menuProvider).error ?? 'unknown'}')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final menus = ref.watch(menuProvider).menus;
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: Text(_isNew ? 'Tambah Item' : 'Edit Item')),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextFormField(
                  controller: _name,
                  decoration: const InputDecoration(labelText: 'Nama item', border: OutlineInputBorder()),
                  validator: (v) => (v == null || v.trim().isEmpty) ? 'Wajib diisi' : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _description,
                  maxLines: 3,
                  decoration: const InputDecoration(labelText: 'Deskripsi', border: OutlineInputBorder()),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: TextFormField(
                        controller: _price,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(labelText: 'Harga (Rp)', border: OutlineInputBorder()),
                        validator: (v) {
                          final p = int.tryParse((v ?? '').trim());
                          if (p == null || p <= 0) return '> 0';
                          return null;
                        },
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextFormField(
                        controller: _stock,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(labelText: 'Stok', border: OutlineInputBorder()),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: _selectedMenuId,
                  decoration: const InputDecoration(labelText: 'Menu', border: OutlineInputBorder()),
                  items: menus.map((m) => DropdownMenuItem(value: m.id, child: Text(m.name))).toList(),
                  onChanged: (v) => setState(() => _selectedMenuId = v),
                ),
                const SizedBox(height: 12),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('Tersedia'),
                  value: _isAvailable,
                  onChanged: (v) => setState(() => _isAvailable = v),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _imageUrl,
                  decoration: const InputDecoration(
                    labelText: 'Image URL',
                    border: OutlineInputBorder(),
                    hintText: 'Tempel URL gambar atau pilih dari galeri',
                  ),
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    OutlinedButton.icon(
                      onPressed: _pickImage,
                      icon: const Icon(Icons.photo_library_outlined),
                      label: const Text('Pilih Gambar'),
                    ),
                    if (_pickedImagePath != null) ...[
                      const SizedBox(width: 12),
                      Expanded(
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(8),
                          child: Image.file(File(_pickedImagePath!), height: 48, fit: BoxFit.cover),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: _submitting ? null : _save,
                  style: FilledButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 16)),
                  child: _submitting
                      ? const SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Simpan'),
                ),
                const SizedBox(height: 8),
                OutlinedButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text('Batal'),
                ),
                if (!_isNew) ...[
                  const SizedBox(height: 8),
                  OutlinedButton.icon(
                    onPressed: _delete,
                    style: OutlinedButton.styleFrom(foregroundColor: theme.colorScheme.error),
                    icon: const Icon(Icons.delete_outline),
                    label: const Text('Hapus Item'),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}