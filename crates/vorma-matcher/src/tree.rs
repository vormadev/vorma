use std::collections::HashMap;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum NodeType {
	Static,
	Dynamic,
	Splat,
}

#[derive(Clone, Debug)]
pub(crate) struct SegmentNode {
	pub(crate) pattern: String,
	pub(crate) node_type: NodeType,
	pub(crate) children: HashMap<String, SegmentNode>,
	pub(crate) dyn_children: Vec<SegmentNode>,
	pub(crate) param_name: String,
	pub(crate) final_score: i32,
}

impl Default for SegmentNode {
	fn default() -> Self {
		Self {
			pattern: String::new(),
			node_type: NodeType::Static,
			children: HashMap::new(),
			dyn_children: Vec::new(),
			param_name: String::new(),
			final_score: 0,
		}
	}
}

impl SegmentNode {
	pub(crate) fn find_or_create_child(&mut self, seg: &str) -> &mut SegmentNode {
		if seg.is_empty() {
			return self
				.children
				.entry(String::new())
				.or_insert_with(|| SegmentNode {
					node_type: NodeType::Static,
					..SegmentNode::default()
				});
		}

		match seg.as_bytes()[0] {
			b':' => {
				let param_name = &seg[1..];
				if let Some(pos) = self.dyn_children.iter().position(|child| {
					child.node_type == NodeType::Dynamic && child.param_name == param_name
				}) {
					return &mut self.dyn_children[pos];
				}
				self.dyn_children.push(SegmentNode {
					node_type: NodeType::Dynamic,
					param_name: param_name.to_string(),
					..SegmentNode::default()
				});
				let len = self.dyn_children.len();
				&mut self.dyn_children[len - 1]
			}
			b'*' => {
				if let Some(pos) = self
					.dyn_children
					.iter()
					.position(|child| child.node_type == NodeType::Splat)
				{
					return &mut self.dyn_children[pos];
				}
				self.dyn_children.push(SegmentNode {
					node_type: NodeType::Splat,
					..SegmentNode::default()
				});
				let len = self.dyn_children.len();
				&mut self.dyn_children[len - 1]
			}
			_ => self
				.children
				.entry(seg.to_string())
				.or_insert_with(|| SegmentNode {
					node_type: NodeType::Static,
					..SegmentNode::default()
				}),
		}
	}
}
